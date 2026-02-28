package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
)

// insertTestSummary inserts a summary document and summaries row, returning
// (docID, summaryID). If createdAt > 0 it overrides the DB default.
func insertTestSummary(t *testing.T, d *sql.DB, text, tags, source string, createdAt int64) (int64, int64) {
	t.Helper()
	res, err := d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, lang, source)
		 VALUES ('summary:' || hex(randomblob(4)), 'summary', ?, NULL, ?)`,
		text, source)
	if err != nil {
		t.Fatalf("insert summary doc: %v", err)
	}
	docID, _ := res.LastInsertId()

	var res2 sql.Result
	if createdAt > 0 {
		res2, err = d.ExecContext(context.Background(),
			`INSERT INTO summaries (doc_id, tags, created_at) VALUES (?, ?, ?)`,
			docID, tags, createdAt)
	} else {
		res2, err = d.ExecContext(context.Background(),
			`INSERT INTO summaries (doc_id, tags) VALUES (?, ?)`,
			docID, tags)
	}
	if err != nil {
		t.Fatalf("insert summary row: %v", err)
	}
	summaryID, _ := res2.LastInsertId()
	return docID, summaryID
}

func TestListSummaries_Empty(t *testing.T) {
	d := openTestDB(t)
	items, err := ListSummaries(context.Background(), d, "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(items)
	if string(b) == "null" {
		t.Error("empty result must marshal to [] not null")
	}
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

func TestListSummaries_ReturnsSummaries(t *testing.T) {
	d := openTestDB(t)
	_, _ = insertTestSummary(t, d, "first summary text", "tag1", "src", 0)
	_, _ = insertTestSummary(t, d, "second summary text", "tag2", "src", 0)

	items, err := ListSummaries(context.Background(), d, "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("want 2 items, got %d", len(items))
	}
	for _, it := range items {
		if it.DocID == 0 {
			t.Error("doc_id should be non-zero")
		}
		if it.Preview == "" {
			t.Error("preview should be non-empty")
		}
	}
}

func TestListSummaries_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	_, _ = insertTestSummary(t, d, "project-a summary", "a", "project-a", 0)
	_, _ = insertTestSummary(t, d, "project-b summary", "b", "project-b", 0)

	items, err := ListSummaries(context.Background(), d, "project-a", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("want 1 item for project-a, got %d", len(items))
	}
	if items[0].Source != "project-a" {
		t.Errorf("want source=project-a, got %q", items[0].Source)
	}
}

func TestListSummaries_ReverseChronological(t *testing.T) {
	d := openTestDB(t)
	// Use explicit timestamps to guarantee deterministic ordering.
	_, id1 := insertTestSummary(t, d, "older summary", "", "src", 1000)
	_, id2 := insertTestSummary(t, d, "newer summary", "", "src", 2000)

	items, err := ListSummaries(context.Background(), d, "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].ID != id2 {
		t.Errorf("want newest (id=%d) first, got id=%d", id2, items[0].ID)
	}
	if items[1].ID != id1 {
		t.Errorf("want oldest (id=%d) second, got id=%d", id1, items[1].ID)
	}
}

func TestDeleteSummary_OK(t *testing.T) {
	d := openTestDB(t)
	docID, summaryID := insertTestSummary(t, d, "to be deleted", "", "src", 0)

	deleted, err := DeleteSummary(context.Background(), d, summaryID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("want deleted=true, got false")
	}
	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM documents WHERE id = ?`, docID).Scan(&count)
	if count != 0 {
		t.Error("document should be deleted after DeleteSummary")
	}
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM summaries WHERE id = ?`, summaryID).Scan(&count)
	if count != 0 {
		t.Error("summaries row should be gone (cascade from documents)")
	}
}

func TestDeleteSummary_NotFound(t *testing.T) {
	d := openTestDB(t)
	deleted, err := DeleteSummary(context.Background(), d, 9999)
	if err != nil {
		t.Fatalf("unexpected error for missing summary: %v", err)
	}
	if deleted {
		t.Error("want deleted=false for missing id, got true")
	}
}

func TestDeleteSummary_CascadesEmbedding(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)
	docID, summaryID := insertTestSummary(t, d, "player game summary content", "", "src", 0)

	if err := EmbedAndStore(context.Background(), d, emb, docID, "player game"); err != nil {
		t.Fatalf("embed: %v", err)
	}
	var count int
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count == 0 {
		t.Fatal("setup: embedding should exist before delete")
	}

	deleted, err := DeleteSummary(context.Background(), d, summaryID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("want deleted=true")
	}
	d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count != 0 {
		t.Error("embedding should be deleted (cascade from documents)")
	}
}
