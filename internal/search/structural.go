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
	Preview   string `json:"preview,omitempty"` // first 5 lines of the function body
}

// Structural searches the symbols table by name (substring match).
// If source is non-empty, results are filtered to that source tag only and
// paths are returned relative to source.
// Preview is populated from the corresponding chunk document when available.
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
			SELECT s.doc_id, d.path, COALESCE(d.source,''), s.kind, s.name,
			       COALESCE(s.signature,''), s.start_line, s.end_line,
			       COALESCE(chunk.content, '')
			FROM symbols s
			JOIN documents d ON d.id = s.doc_id
			LEFT JOIN documents chunk ON chunk.kind = 'chunk'
			    AND chunk.parent_id = s.doc_id
			    AND chunk.path = d.path || '::' || s.name || '@' || CAST(s.start_line AS TEXT)
			WHERE s.name LIKE ? AND d.source = ?
			ORDER BY CASE WHEN s.name = ? THEN 0 ELSE 1 END, s.name
			LIMIT ?
		`, "%"+symbol+"%", source, symbol, limit)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT s.doc_id, d.path, COALESCE(d.source,''), s.kind, s.name,
			       COALESCE(s.signature,''), s.start_line, s.end_line,
			       COALESCE(chunk.content, '')
			FROM symbols s
			JOIN documents d ON d.id = s.doc_id
			LEFT JOIN documents chunk ON chunk.kind = 'chunk'
			    AND chunk.parent_id = s.doc_id
			    AND chunk.path = d.path || '::' || s.name || '@' || CAST(s.start_line AS TEXT)
			WHERE s.name LIKE ?
			ORDER BY CASE WHEN s.name = ? THEN 0 ELSE 1 END, s.name
			LIMIT ?
		`, "%"+symbol+"%", symbol, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]SymbolResult, 0)
	for rows.Next() {
		var r SymbolResult
		var body string
		if err := rows.Scan(&r.DocID, &r.Path, &r.Source, &r.Kind, &r.Name, &r.Signature, &r.StartLine, &r.EndLine, &body); err != nil {
			return nil, err
		}
		r.Path = relativizePath(r.Path, source)
		r.Preview = firstLines(body, 20)
		results = append(results, r)
	}
	return results, rows.Err()
}
