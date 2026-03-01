package search

import (
	"context"
	"database/sql"
	"strings"
)

// SymbolContent is the full body of a named symbol returned by ReadSymbol.
type SymbolContent struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// ReadSymbol returns the full content of a named symbol within a file.
// filePath and source follow the same convention as other search endpoints:
// filePath is relative to source (which is the absolute indexed root).
// Returns nil, nil when no matching symbol is found.
func ReadSymbol(ctx context.Context, db *sql.DB, filePath, name, source string) (*SymbolContent, error) {
	absPath := absFilePath(filePath, source)
	row := db.QueryRowContext(ctx, `
		SELECT COALESCE(chunk.content, ''), s.kind, s.start_line, s.end_line
		FROM symbols s
		JOIN documents d ON d.id = s.doc_id
		LEFT JOIN documents chunk
		    ON chunk.path = d.path || '::' || s.name || '@' || CAST(s.start_line AS TEXT)
		WHERE s.name = ? AND d.path = ?
		LIMIT 1
	`, name, absPath)

	var sc SymbolContent
	sc.Path = filePath
	sc.Name = name
	if err := row.Scan(&sc.Content, &sc.Kind, &sc.StartLine, &sc.EndLine); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sc, nil
}

// ContextResult is a window of lines from an indexed file returned by ReadContext.
type ContextResult struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// ReadContext returns n lines above and below the given 1-based line number
// from the stored file document. filePath and source follow the same convention
// as other endpoints. Returns nil, nil when the file is not in the index.
func ReadContext(ctx context.Context, db *sql.DB, filePath string, line, n int, source string) (*ContextResult, error) {
	absPath := absFilePath(filePath, source)
	var raw string
	err := db.QueryRowContext(ctx,
		`SELECT content FROM documents WHERE path = ? AND kind = 'file'`, absPath,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	lines := strings.Split(raw, "\n")
	total := len(lines)

	start := line - n
	if start < 1 {
		start = 1
	}
	end := line + n
	if end > total {
		end = total
	}

	return &ContextResult{
		Path:      filePath,
		StartLine: start,
		EndLine:   end,
		Content:   strings.Join(lines[start-1:end], "\n"),
	}, nil
}

func absFilePath(relPath, source string) string {
	if source == "" {
		return relPath
	}
	return strings.TrimRight(source, "/") + "/" + relPath
}
