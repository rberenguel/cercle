package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ruben/cercle/internal/indexer"
	"github.com/ruben/cercle/internal/search"
)

func queryFloat(r *http.Request, key string, def float64) float64 {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return def
}

// -- helpers ------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func queryInt(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return def
}

// -- POST /index --------------------------------------------------------------

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Path   string `json:"path"`
		Source string `json:"source"` // agent/project identifier; defaults to abs(path)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "body must contain {\"path\": \"<dir>\"}")
		return
	}

	result, err := indexer.IndexDir(r.Context(), s.db, req.Path, req.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Trigger the embed worker so new documents get embedded promptly.
	s.worker.Trigger()

	pending, _ := search.PendingCount(r.Context(), s.db, s.emb)
	writeJSON(w, http.StatusOK, map[string]any{
		"files":           result.Files,
		"symbols":         result.Symbols,
		"skipped":         result.Skipped,
		"errors":          result.Errors,
		"pending_embed":   pending,
	})
}

// -- POST /embed --------------------------------------------------------------

func (s *Server) handleEmbed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	pending, err := search.PendingCount(r.Context(), s.db, s.emb)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.worker.Trigger()
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":        "triggered",
		"pending_embed": pending,
	})
}

// -- GET /search/lexical?q=<query>[&limit=N] ----------------------------------

func (s *Server) handleLexical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query param 'q'")
		return
	}
	results, err := search.Lexical(r.Context(), s.db, q, r.URL.Query().Get("source"), queryInt(r, "limit", 10))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// -- GET /search/semantic?q=<concept>[&limit=N] -------------------------------

func (s *Server) handleSemantic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query param 'q'")
		return
	}
	results, err := search.Semantic(r.Context(), s.db, s.emb, q, r.URL.Query().Get("source"), queryInt(r, "limit", 10), queryFloat(r, "min_similarity", 0))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// -- GET /search/structural?q=<symbol>[&limit=N] ------------------------------

func (s *Server) handleStructural(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query param 'q'")
		return
	}
	results, err := search.Structural(r.Context(), s.db, q, r.URL.Query().Get("source"), queryInt(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// -- /summary (POST = create, DELETE = remove) --------------------------------

func (s *Server) routeSummary(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleSummary(w, r)
	case http.MethodDelete:
		s.handleDeleteSummary(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "POST or DELETE required")
	}
}

// -- POST /summary ------------------------------------------------------------

type summaryRequest struct {
	Tags   string `json:"tags"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req summaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		writeError(w, http.StatusBadRequest, "body must contain {\"tags\": \"...\", \"text\": \"...\"}")
		return
	}

	// Insert as a document (kind=summary) so FTS5 trigger fires automatically.
	// Use a pseudo-path based on current time to avoid collisions.
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(r.Context(), `
		INSERT INTO documents (path, kind, content, lang, source)
		VALUES ('summary:' || unixepoch() || ':' || substr(hex(randomblob(4)),1,8), 'summary', ?, NULL, ?)
	`, req.Text, nullableString(req.Source))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	docID, _ := res.LastInsertId()

	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO summaries (doc_id, tags) VALUES (?, ?)
	`, docID, req.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Best-effort: generate and store embedding (no-op if vectors not loaded).
	go search.EmbedAndStore(r.Context(), s.db, s.emb, docID, req.Text) //nolint:errcheck

	writeJSON(w, http.StatusCreated, map[string]any{"id": docID, "status": "indexed"})
}

// -- GET /summaries?[source=...][&limit=N] ------------------------------------

func (s *Server) handleSummaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	items, err := search.ListSummaries(r.Context(), s.db, r.URL.Query().Get("source"), queryInt(r, "limit", 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// -- DELETE /summary?id=N -----------------------------------------------------
// Extends the existing /summary route; POST is handled by handleSummary above.

func (s *Server) handleDeleteSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing query param 'id'")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	deleted, err := search.DeleteSummary(r.Context(), s.db, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "summary not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "deleted"})
}

// -- DELETE /source?source=<ns> -----------------------------------------------

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing query param 'source'")
		return
	}
	n, err := search.DeleteSource(r.Context(), s.db, source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": source, "deleted": n})
}

// -- GET /files?[source=...][&limit=N] ----------------------------------------

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	source := r.URL.Query().Get("source")
	limit := queryInt(r, "limit", 0) // 0 = no limit

	files, err := search.Files(r.Context(), s.db, source, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}
