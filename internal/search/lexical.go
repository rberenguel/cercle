package search

import (
	"context"
	"database/sql"
	"strings"
)

// LexicalResult is a single FTS5 hit.
type LexicalResult struct {
	ID      int64   `json:"id"`
	Path    string  `json:"path"`
	Source  string  `json:"source"`
	Snippet string  `json:"snippet"`
	Rank    float64 `json:"rank"`
}

// SanitizeFTSQuery strips FTS5 operator characters so that plain user input
// (e.g. "os.Exit", "command-line") does not trigger FTS5 syntax errors.
// Keeps letters, digits, underscores, and spaces; replaces everything else
// (., -, *, ", (, ), :, ^, ~, {, }, [, ], +, /) with a space.
func SanitizeFTSQuery(q string) string {
	var sb strings.Builder
	for _, r := range q {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_',
			r == ' ':
			sb.WriteRune(r)
		default:
			sb.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// Lexical runs a full-text search against the FTS5 index.
// Unless raw is true, the query is sanitized to prevent FTS5 syntax errors
// from user input that contains special characters like ".", "-", "*", etc.
// If source is non-empty, results are filtered to that source tag only and
// paths are returned relative to source.
// When both a file and one of its chunks match, the parent file is suppressed.
func Lexical(ctx context.Context, db *sql.DB, query, source string, limit int, raw bool) ([]LexicalResult, error) {
	if limit <= 0 {
		limit = 10
	}

	q := query
	if !raw {
		q = SanitizeFTSQuery(query)
	}
	if q == "" {
		return make([]LexicalResult, 0), nil
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
				snippet(documents_fts, 1, '', '', '…', 32),
				documents_fts.rank
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.rowid
			WHERE documents_fts MATCH ? AND d.source = ?
			ORDER BY rank
			LIMIT ?
		`, q, source, limit)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT
				d.id,
				d.path,
				COALESCE(d.source, ''),
				snippet(documents_fts, 1, '', '', '…', 32),
				documents_fts.rank
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.rowid
			WHERE documents_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`, q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	raw2 := make([]LexicalResult, 0)
	for rows.Next() {
		var r LexicalResult
		if err := rows.Scan(&r.ID, &r.Path, &r.Source, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		r.Path = relativizePath(r.Path, source)
		raw2 = append(raw2, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Suppress parent-file results when a chunk from the same file also matched.
	chunkParents := make(map[string]bool)
	for _, r := range raw2 {
		if idx := strings.Index(r.Path, "::"); idx != -1 {
			chunkParents[r.Path[:idx]] = true
		}
	}
	results := make([]LexicalResult, 0, len(raw2))
	for _, r := range raw2 {
		if !strings.Contains(r.Path, "::") && chunkParents[r.Path] {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}
