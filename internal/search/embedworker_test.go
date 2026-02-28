package search

import (
	"context"
	"testing"
)

func TestEmbedWorker_SkipsFilesWithChunks(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	// Insert a file document with symbols (represented by a child chunk).
	fileID := insertDoc(t, d, "/a/main.go", "player game code", "src")
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, lang, source, parent_id)
		 VALUES ('/a/main.go::Func@1', 'chunk', 'player game', NULL, 'src', ?)`,
		fileID)
	if err != nil {
		t.Fatalf("insert chunk: %v", err)
	}

	w := NewEmbedWorker(d, emb)
	w.process(context.Background())

	// The file document (kind='file' with chunks) must NOT be embedded.
	var fileEmbedCount int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, fileID).Scan(&fileEmbedCount)
	if fileEmbedCount != 0 {
		t.Errorf("file with chunks should not be embedded, got %d embedding(s)", fileEmbedCount)
	}

	// The chunk document MUST be embedded.
	var chunkID int64
	d.QueryRowContext(context.Background(),
		`SELECT id FROM documents WHERE kind = 'chunk'`).Scan(&chunkID)
	var chunkEmbedCount int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, chunkID).Scan(&chunkEmbedCount)
	if chunkEmbedCount == 0 {
		t.Error("chunk document should be embedded")
	}
}

func TestEmbedWorker_EmbedsFilesWithoutChunks(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	// Markdown file: no symbols → no chunks → should be embedded as whole document.
	docID := insertDoc(t, d, "/README.md", "player game overview document", "src")

	w := NewEmbedWorker(d, emb)
	w.process(context.Background())

	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count == 0 {
		t.Error("file without chunks should be embedded")
	}
}
