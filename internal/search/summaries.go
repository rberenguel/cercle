package search

import (
	"context"
	"database/sql"
)

// SummaryItem is a single summary listing entry.
type SummaryItem struct {
	ID        int64  `json:"id"`
	DocID     int64  `json:"doc_id"`
	Tags      string `json:"tags"`
	CreatedAt int64  `json:"created_at"`
	Source    string `json:"source"`
	Text      string `json:"text"`
}

// ListSummaries returns summaries in reverse chronological order.
// source filters by namespace; empty returns all.
func ListSummaries(ctx context.Context, db *sql.DB, source string, limit int) ([]SummaryItem, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows *sql.Rows
		err  error
	)
	if source != "" {
		rows, err = db.QueryContext(ctx, `
			SELECT s.id, s.doc_id, COALESCE(s.tags,''), s.created_at,
			       COALESCE(d.source,''), d.content
			FROM summaries s
			JOIN documents d ON d.id = s.doc_id
			WHERE d.source = ?
			ORDER BY s.created_at DESC
			LIMIT ?
		`, source, limit)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT s.id, s.doc_id, COALESCE(s.tags,''), s.created_at,
			       COALESCE(d.source,''), d.content
			FROM summaries s
			JOIN documents d ON d.id = s.doc_id
			ORDER BY s.created_at DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SummaryItem, 0)
	for rows.Next() {
		var it SummaryItem
		if err := rows.Scan(&it.ID, &it.DocID, &it.Tags, &it.CreatedAt, &it.Source, &it.Text); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// DeleteSummary removes a summary and its underlying document (cascades to
// FTS5, embeddings, and the summaries row itself).
// Returns (true, nil) if deleted, (false, nil) if not found.
func DeleteSummary(ctx context.Context, db *sql.DB, summaryID int64) (bool, error) {
	var docID int64
	err := db.QueryRowContext(ctx,
		`SELECT doc_id FROM summaries WHERE id = ?`, summaryID).Scan(&docID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, docID)
	return err == nil, err
}
