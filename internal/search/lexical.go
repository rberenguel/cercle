package search

import (
	"context"
	"database/sql"
)

// LexicalResult is a single FTS5 hit.
type LexicalResult struct {
	ID      int64   `json:"id"`
	Path    string  `json:"path"`
	Source  string  `json:"source"`
	Snippet string  `json:"snippet"`
	Rank    float64 `json:"rank"`
}

// Lexical runs a full-text search against the FTS5 index.
// If source is non-empty, results are filtered to that source tag only.
func Lexical(ctx context.Context, db *sql.DB, query, source string, limit int) ([]LexicalResult, error) {
	if limit <= 0 {
		limit = 10
	}

	var (
		rows *sql.Rows
		err  error
	)
	if source != "" {
		rows, err = db.QueryContext(ctx, `
			SELECT
				d.id,
				d.path,
				COALESCE(d.source, ''),
				snippet(documents_fts, 1, '**', '**', '…', 32),
				documents_fts.rank
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.rowid
			WHERE documents_fts MATCH ? AND d.source = ?
			ORDER BY rank
			LIMIT ?
		`, query, source, limit)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT
				d.id,
				d.path,
				COALESCE(d.source, ''),
				snippet(documents_fts, 1, '**', '**', '…', 32),
				documents_fts.rank
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.rowid
			WHERE documents_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]LexicalResult, 0)
	for rows.Next() {
		var r LexicalResult
		if err := rows.Scan(&r.ID, &r.Path, &r.Source, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
