package search

import (
	"context"
	"database/sql"
)

// Files returns all indexed file paths, optionally filtered by source.
// limit=0 means no limit.
func Files(ctx context.Context, db *sql.DB, source string, limit int) ([]string, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if source != "" && limit > 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT path FROM documents WHERE kind = 'file' AND source = ? ORDER BY path LIMIT ?`,
			source, limit)
	} else if source != "" {
		rows, err = db.QueryContext(ctx,
			`SELECT path FROM documents WHERE kind = 'file' AND source = ? ORDER BY path`,
			source)
	} else if limit > 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT path FROM documents WHERE kind = 'file' ORDER BY path LIMIT ?`,
			limit)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT path FROM documents WHERE kind = 'file' ORDER BY path`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
