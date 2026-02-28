package search

import (
	"context"
	"testing"
)

func TestDeleteSource_RemovesDocuments(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/main.go", "content", "project-a")
	insertDoc(t, d, "/a/util.go", "content", "project-a")

	n, err := DeleteSource(context.Background(), d, "project-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 deleted, got %d", n)
	}

	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM documents WHERE source = 'project-a'`).Scan(&count)
	if count != 0 {
		t.Errorf("want 0 documents remaining, got %d", count)
	}
}

func TestDeleteSource_LeavesOtherSources(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/main.go", "content", "project-a")
	insertDoc(t, d, "/b/main.go", "content", "project-b")

	_, err := DeleteSource(context.Background(), d, "project-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM documents WHERE source = 'project-b'`).Scan(&count)
	if count != 1 {
		t.Errorf("want project-b document intact, got %d", count)
	}
}

func TestDeleteSource_CascadesEmbeddings(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)
	docID := insertDoc(t, d, "/a/main.go", "player game content", "project-a")
	if err := EmbedAndStore(context.Background(), d, emb, docID, "player game"); err != nil {
		t.Fatalf("embed: %v", err)
	}

	_, err := DeleteSource(context.Background(), d, "project-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count != 0 {
		t.Errorf("embedding should be cascade-deleted, got %d", count)
	}
}

func TestDeleteSource_EmptySource(t *testing.T) {
	d := openTestDB(t)
	n, err := DeleteSource(context.Background(), d, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("want 0 deleted for unknown source, got %d", n)
	}
}
