package search

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/ruben/cercle/internal/db"
	"github.com/ruben/cercle/internal/embedder"
)

// openTestDB returns an in-memory SQLite DB with the full schema applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// insertDoc inserts a file document and returns its ID.
func insertDoc(t *testing.T, d *sql.DB, path, content, source string) int64 {
	t.Helper()
	res, err := d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, lang, source) VALUES (?, 'file', ?, NULL, ?)`,
		path, content, source)
	if err != nil {
		t.Fatalf("insert doc %q: %v", path, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// testEmbedder builds a minimal 4-dim embedder from inline vocab.
// Orthogonal unit vectors so similarity scores are predictable:
//
//	player  = [1, 0, 0, 0]
//	game    = [0, 1, 0, 0]
//	enemy   = [0, 0, 1, 0]
//	physics = [0, 0, 0, 1]
func testEmbedder(t *testing.T) *embedder.Embedder {
	t.Helper()
	vocab := strings.NewReader(
		"player 1.0 0.0 0.0 0.0\n" +
			"game 0.0 1.0 0.0 0.0\n" +
			"enemy 0.0 0.0 1.0 0.0\n" +
			"physics 0.0 0.0 0.0 1.0\n",
	)
	emb, err := embedder.Load(vocab)
	if err != nil {
		t.Fatalf("load test embedder: %v", err)
	}
	return emb
}
