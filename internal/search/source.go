package search

import (
	"context"
	"database/sql"
)

// DeleteSource removes all documents (files, chunks, summaries) for the given
// source namespace. Cascades to embeddings, symbols, FTS5, and summaries rows.
// Returns the number of document rows deleted.
func DeleteSource(ctx context.Context, db *sql.DB, source string) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM documents WHERE source = ?`, source)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
