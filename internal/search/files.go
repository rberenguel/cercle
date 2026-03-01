package search

import (
	"context"
	"database/sql"
)

// SourceInfo summarises one indexed source namespace.
type SourceInfo struct {
	Source   string `json:"source"`
	Files    int    `json:"files"`
	Chunks   int    `json:"chunks"`
	Embedded int    `json:"embedded"`
	Pending  int    `json:"pending"`
}

// Sources returns all distinct source namespaces with depth stats.
func Sources(ctx context.Context, db *sql.DB) ([]SourceInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(d.source, '')                                                   AS source,
			COUNT(CASE WHEN d.kind = 'file'  THEN 1 END)                            AS files,
			COUNT(CASE WHEN d.kind = 'chunk' THEN 1 END)                            AS chunks,
			COUNT(CASE WHEN e.id IS NOT NULL AND length(e.vector) > 0 THEN 1 END)   AS embedded,
			COUNT(CASE WHEN e.id IS NULL
			           AND NOT (d.kind = 'file' AND EXISTS (
			               SELECT 1 FROM documents c WHERE c.parent_id = d.id
			           ))
			      THEN 1 END)                                                        AS pending
		FROM documents d
		LEFT JOIN embeddings e ON e.doc_id = d.id
		WHERE d.kind IN ('file', 'chunk')
		GROUP BY d.source
		ORDER BY d.source
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SourceInfo, 0)
	for rows.Next() {
		var s SourceInfo
		if err := rows.Scan(&s.Source, &s.Files, &s.Chunks, &s.Embedded, &s.Pending); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Files returns all indexed file paths, optionally filtered by source and/or
// a SQLite GLOB pattern on the path. limit=0 means no limit.
func Files(ctx context.Context, db *sql.DB, source, glob string, limit int) ([]string, error) {
	query := `SELECT path FROM documents WHERE kind = 'file'`
	args := []any{}

	if source != "" {
		query += ` AND source = ?`
		args = append(args, source)
	}
	if glob != "" {
		query += ` AND path GLOB ?`
		args = append(args, glob)
	}
	query += ` ORDER BY path`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
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
		paths = append(paths, relativizePath(p, source))
	}
	return paths, rows.Err()
}
