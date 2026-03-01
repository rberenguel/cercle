package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruben/cercle/internal/db"
	"github.com/ruben/cercle/internal/search"
)

// newTestServer wires up a Server backed by an in-memory SQLite DB.
// emb is nil (semantic search disabled) unless the test needs it.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	worker := search.NewEmbedWorker(d, nil)
	return NewServer("", d, nil, worker, "test")
}

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func post(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

// ---- /health ----

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/health")

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %v", body["status"])
	}
	// semantic should be false because we passed nil embedder.
	if body["semantic"] != false {
		t.Errorf("want semantic=false (nil embedder), got %v", body["semantic"])
	}
}

// ---- /search/lexical ----

func TestHandleLexical_MissingQ(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/search/lexical")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleLexical_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/search/lexical", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

func TestHandleLexical_EmptyResultsIsArray(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/search/lexical?q=zzznomatch")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Results []any  `json:"results"`
		Query   string `json:"query"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Results == nil {
		t.Error("want results array, got nil")
	}
	if body.Query == "" {
		t.Error("want non-empty query field")
	}
}

// ---- /search/structural ----

func TestHandleStructural_MissingQ(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/search/structural")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleStructural_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/search/structural", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

func TestHandleStructural_EmptyResultsIsArray(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/search/structural?q=NoSuchSymbol")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) == "null" {
		t.Error("want [], got null")
	}
}

// ---- /search/semantic ----

func TestHandleSemantic_MissingQ(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/search/semantic")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleSemantic_NoVectors(t *testing.T) {
	// Embedder is nil — should return 500 with an error message.
	s := newTestServer(t)
	w := get(t, s, "/search/semantic?q=player")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 when no vectors loaded, got %d", w.Code)
	}
}

func TestHandleSemantic_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/search/semantic", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

// ---- /files ----

func TestHandleFiles_OK(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/files")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if _, ok := body["files"]; !ok {
		t.Error("response missing 'files' key")
	}
}

func TestHandleFiles_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/files", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

func del(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

// ---- /summaries ----

func TestHandleSummaries_OK(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/summaries")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) == "null" {
		t.Error("want [], got null")
	}
}

func TestHandleSummaries_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/summaries", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

// ---- /summary DELETE ----

func TestHandleDeleteSummary_MissingID(t *testing.T) {
	s := newTestServer(t)
	w := del(t, s, "/summary")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing id, got %d", w.Code)
	}
}

func TestHandleDeleteSummary_NotFound(t *testing.T) {
	s := newTestServer(t)
	w := del(t, s, "/summary?id=9999")
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for unknown summary id, got %d", w.Code)
	}
}

func TestHandleDeleteSummary_OK(t *testing.T) {
	s := newTestServer(t)

	// Create a summary.
	wc := post(t, s, "/summary", `{"tags":"test","text":"hello world","source":"src"}`)
	if wc.Code != http.StatusCreated {
		t.Fatalf("create summary: want 201, got %d: %s", wc.Code, wc.Body.String())
	}

	// Retrieve its summary ID via GET /summaries.
	wl := get(t, s, "/summaries")
	var items []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(wl.Body.Bytes(), &items); err != nil {
		t.Fatalf("parse summaries: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one summary after POST")
	}

	// Delete it.
	wd := del(t, s, fmt.Sprintf("/summary?id=%d", items[0].ID))
	if wd.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", wd.Code, wd.Body.String())
	}

	// Confirm it's gone.
	wl2 := get(t, s, "/summaries")
	var after []any
	json.Unmarshal(wl2.Body.Bytes(), &after)
	if len(after) != 0 {
		t.Errorf("want 0 summaries after delete, got %d", len(after))
	}
}

// ---- /source ----

func TestHandleDeleteSource_MissingSource(t *testing.T) {
	s := newTestServer(t)
	w := del(t, s, "/source")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing source, got %d", w.Code)
	}
}

func TestHandleDeleteSource_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/source?source=x")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

func TestHandleDeleteSource_OK(t *testing.T) {
	s := newTestServer(t)

	// Index two files under a source so there is something to delete.
	post(t, s, "/index", `{"path":"`+t.TempDir()+`","source":"proj"}`)

	w := del(t, s, "/source?source=proj")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body["source"] != "proj" {
		t.Errorf("want source=proj in response, got %v", body["source"])
	}
	if _, ok := body["deleted"]; !ok {
		t.Error("response missing 'deleted' key")
	}
}

// ---- /index ----

func TestHandleIndex_BadBody(t *testing.T) {
	s := newTestServer(t)
	w := post(t, s, "/index", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing path, got %d", w.Code)
	}
}

func TestHandleIndex_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	w := get(t, s, "/index")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}
