package indexer

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Result summarises a completed indexing run.
type Result struct {
	Files    int
	Symbols  int
	Skipped  int
	Errors   []string
}

// IndexDir walks root, ingests every supported file, and extracts symbols.
// source is an opaque tag identifying the agent/project (e.g. $PWD); if empty,
// the absolute root path is used so documents are always namespaced.
func IndexDir(ctx context.Context, db *sql.DB, root, source string) (*Result, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if source == "" {
		source = abs
	}

	res := &Result{}
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("walk %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSupportedFile(d.Name()) {
			res.Skipped++
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("read %s: %v", path, err))
			return nil
		}

		lang := detectLang(d.Name())
		docID, err := upsertDocument(ctx, db, path, "file", string(content), lang, source, 0)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("upsert %s: %v", path, err))
			return nil
		}
		res.Files++

		if lang != "" {
			syms, err := extractSymbols(content, lang)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("parse %s: %v", path, err))
				return nil
			}
			if err := insertSymbols(ctx, db, docID, syms); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("symbols %s: %v", path, err))
				return nil
			}
			if err := insertChunks(ctx, db, docID, path, content, syms, lang, source); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("chunks %s: %v", path, err))
				return nil
			}
			res.Symbols += len(syms)
		}

		return nil
	})

	return res, err
}

// insertChunks emits one chunk document per extracted symbol, keyed by
// "{path}::{name}@{start_line}". Chunks that no longer exist after a re-index
// are pruned. The parent file document is intentionally left without an
// embedding when chunks exist; the embed worker skips it automatically.
func insertChunks(ctx context.Context, db *sql.DB, parentID int64, filePath string, content []byte, syms []Symbol, lang, source string) error {
	newPaths := make([]string, 0, len(syms))

	for _, sym := range syms {
		chunkContent := extractChunkContent(content, sym)
		if chunkContent == "" {
			continue
		}
		chunkPath := fmt.Sprintf("%s::%s@%d", filePath, sym.Name, sym.StartLine)
		newPaths = append(newPaths, chunkPath)

		_, err := db.ExecContext(ctx, `
			INSERT INTO documents (path, kind, content, lang, source, parent_id)
			VALUES (?, 'chunk', ?, ?, ?, ?)
			ON CONFLICT(path) DO UPDATE SET
				content    = excluded.content,
				lang       = excluded.lang,
				source     = excluded.source,
				parent_id  = excluded.parent_id,
				indexed_at = unixepoch()
		`, chunkPath, chunkContent, nullableString(lang), nullableString(source), parentID)
		if err != nil {
			return fmt.Errorf("upsert chunk %s: %w", chunkPath, err)
		}
	}

	// Prune chunks that no longer exist (symbol was deleted or renamed).
	if len(newPaths) > 0 {
		placeholders := strings.Repeat("?,", len(newPaths))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, 1+len(newPaths))
		args[0] = parentID
		for i, p := range newPaths {
			args[i+1] = p
		}
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM documents WHERE parent_id = ? AND path NOT IN (%s)`, placeholders),
			args...)
		if err != nil {
			return fmt.Errorf("prune stale chunks: %w", err)
		}
	} else {
		_, err := db.ExecContext(ctx, `DELETE FROM documents WHERE parent_id = ?`, parentID)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractChunkContent returns the source lines for a symbol (start_line..end_line, 1-indexed).
func extractChunkContent(content []byte, sym Symbol) string {
	lines := bytes.Split(content, []byte("\n"))
	start := int(sym.StartLine) - 1
	end := int(sym.EndLine)
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return ""
	}
	return string(bytes.Join(lines[start:end], []byte("\n")))
}

func upsertDocument(ctx context.Context, db *sql.DB, path, kind, content, lang, source string, parentID int64) (int64, error) {
	var res sql.Result
	var err error
	if parentID != 0 {
		res, err = db.ExecContext(ctx, `
			INSERT INTO documents (path, kind, content, lang, source, parent_id)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(path) DO UPDATE SET
				content    = excluded.content,
				lang       = excluded.lang,
				source     = excluded.source,
				parent_id  = excluded.parent_id,
				indexed_at = unixepoch()
		`, path, kind, content, nullableString(lang), nullableString(source), parentID)
	} else {
		res, err = db.ExecContext(ctx, `
			INSERT INTO documents (path, kind, content, lang, source)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(path) DO UPDATE SET
				content    = excluded.content,
				lang       = excluded.lang,
				source     = excluded.source,
				indexed_at = unixepoch()
		`, path, kind, content, nullableString(lang), nullableString(source))
	}
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		// ON CONFLICT branch: fetch existing id
		err = db.QueryRowContext(ctx, `SELECT id FROM documents WHERE path = ?`, path).Scan(&id)
	}
	return id, err
}

func insertSymbols(ctx context.Context, db *sql.DB, docID int64, syms []Symbol) error {
	// Clear old symbols for this document before re-inserting.
	if _, err := db.ExecContext(ctx, `DELETE FROM symbols WHERE doc_id = ?`, docID); err != nil {
		return err
	}
	for _, s := range syms {
		_, err := db.ExecContext(ctx, `
			INSERT INTO symbols (doc_id, kind, name, signature, start_line, end_line)
			VALUES (?, ?, ?, ?, ?, ?)
		`, docID, s.Kind, s.Name, nullableString(s.Signature), s.StartLine, s.EndLine)
		if err != nil {
			return err
		}
	}
	return nil
}

func shouldSkipDir(name string) bool {
	skip := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".venv": true, "__pycache__": true, "dist": true, "build": true,
	}
	return skip[name]
}

func isSupportedFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	supported := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
		".sh": true, ".toml": true, ".rs": true, ".c": true, ".h": true,
	}
	return supported[ext]
}

func detectLang(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	langs := map[string]string{
		".go": "go", ".py": "python", ".js": "javascript",
		".ts": "javascript", ".rs": "rust", ".c": "c", ".h": "c",
	}
	return langs[ext]
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
