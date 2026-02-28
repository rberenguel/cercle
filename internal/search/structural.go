package search

import (
	"context"
	"database/sql"
)

// SymbolResult is a single structural hit.
type SymbolResult struct {
	DocID     int64  `json:"doc_id"`
	Path      string `json:"path"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// Structural searches the symbols table by name (prefix match).
// If source is non-empty, results are filtered to that source tag only.
func Structural(ctx context.Context, db *sql.DB, symbol, source string, limit int) ([]SymbolResult, error) {
	if limit <= 0 {
		limit = 20
	}

	var (
		rows *sql.Rows
		err  error
	)
	if source != "" {
		rows, err = db.QueryContext(ctx, `
			SELECT s.doc_id, d.path, COALESCE(d.source,''), s.kind, s.name, COALESCE(s.signature,''), s.start_line, s.end_line
			FROM symbols s
			JOIN documents d ON d.id = s.doc_id
			WHERE s.name LIKE ? AND d.source = ?
			ORDER BY s.name
			LIMIT ?
		`, symbol+"%", source, limit)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT s.doc_id, d.path, COALESCE(d.source,''), s.kind, s.name, COALESCE(s.signature,''), s.start_line, s.end_line
			FROM symbols s
			JOIN documents d ON d.id = s.doc_id
			WHERE s.name LIKE ?
			ORDER BY s.name
			LIMIT ?
		`, symbol+"%", limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]SymbolResult, 0)
	for rows.Next() {
		var r SymbolResult
		if err := rows.Scan(&r.DocID, &r.Path, &r.Source, &r.Kind, &r.Name, &r.Signature, &r.StartLine, &r.EndLine); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
