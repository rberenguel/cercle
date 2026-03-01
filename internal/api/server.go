package api

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/ruben/cercle/internal/embedder"
	"github.com/ruben/cercle/internal/search"
)

//go:embed ui
var uiFiles embed.FS

// Server is the HTTP daemon.
type Server struct {
	addr    string
	db      *sql.DB
	emb     *embedder.Embedder // nil if no vectors loaded
	worker  *search.EmbedWorker
	version string
	mux     *http.ServeMux
}

// NewServer wires up all routes and returns the server.
func NewServer(addr string, db *sql.DB, emb *embedder.Embedder, worker *search.EmbedWorker, version string) *Server {
	s := &Server{addr: addr, db: db, emb: emb, worker: worker, version: version, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Start begins listening. Blocks until the server exits.
func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) routes() {
	uiContent, _ := fs.Sub(uiFiles, "ui")
	s.mux.Handle("/ui/", http.StripPrefix("/ui", http.FileServer(http.FS(uiContent))))

	s.mux.HandleFunc("/sources", s.handleSources)
	s.mux.HandleFunc("/index", s.handleIndex)
	s.mux.HandleFunc("/files", s.handleFiles)
	s.mux.HandleFunc("/search/lexical", s.handleLexical)
	s.mux.HandleFunc("/search/semantic", s.handleSemantic)
	s.mux.HandleFunc("/search/structural", s.handleStructural)
	s.mux.HandleFunc("/symbol", s.handleSymbol)
	s.mux.HandleFunc("/context", s.handleContext)
	s.mux.HandleFunc("/source", s.handleDeleteSource)
	s.mux.HandleFunc("/summary", s.routeSummary)
	s.mux.HandleFunc("/summaries", s.handleSummaries)
	s.mux.HandleFunc("/embed", s.handleEmbed)
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		semantic := s.emb != nil
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","version":%q,"semantic":%v}`, s.version, semantic)
	})
}
