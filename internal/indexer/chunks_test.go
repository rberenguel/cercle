package indexer

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestInsertChunks_EmitsOneChunkPerSymbol(t *testing.T) {
	src := []byte(`package main

func Hello() string { return "hi" }

func World() {}
`)
	d := openTestDB(t)
	docID := mustInsertFile(t, d, "/a/main.go", string(src), "src")

	syms, _ := extractSymbols(src, "go")
	if err := insertChunks(context.Background(), d, docID, "/a/main.go", src, syms, "go", "src"); err != nil {
		t.Fatalf("insertChunks: %v", err)
	}

	var count int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='chunk' AND parent_id=?`, docID).Scan(&count)
	if count != len(syms) {
		t.Errorf("want %d chunks (one per symbol), got %d", len(syms), count)
	}
}

func TestInsertChunks_ChunkPathIsDescriptive(t *testing.T) {
	src := []byte(`package main
func Foo() {}
`)
	d := openTestDB(t)
	docID := mustInsertFile(t, d, "/a/main.go", string(src), "src")
	syms, _ := extractSymbols(src, "go")
	insertChunks(context.Background(), d, docID, "/a/main.go", src, syms, "go", "src")

	var path string
	d.QueryRowContext(context.Background(), `SELECT path FROM documents WHERE kind='chunk'`).Scan(&path)
	if !strings.HasPrefix(path, "/a/main.go::") {
		t.Errorf("chunk path should start with file path, got %q", path)
	}
	if !strings.Contains(path, "Foo") {
		t.Errorf("chunk path should contain symbol name, got %q", path)
	}
}

func TestInsertChunks_ContentIsSymbolSource(t *testing.T) {
	src := []byte(`package main

func Hello() string {
	return "hi"
}

func World() {}
`)
	d := openTestDB(t)
	docID := mustInsertFile(t, d, "/a/main.go", string(src), "src")
	syms, _ := extractSymbols(src, "go")
	insertChunks(context.Background(), d, docID, "/a/main.go", src, syms, "go", "src")

	rows, _ := d.QueryContext(context.Background(), `SELECT path, content FROM documents WHERE kind='chunk'`)
	defer rows.Close()
	for rows.Next() {
		var path, content string
		rows.Scan(&path, &content)
		if strings.Contains(path, "Hello") {
			if !strings.Contains(content, "return") {
				t.Errorf("Hello chunk should contain function body, got %q", content)
			}
			if strings.Contains(content, "World") {
				t.Errorf("Hello chunk should not contain World function, got %q", content)
			}
		}
	}
}

func TestInsertChunks_PrunesDeletedSymbols(t *testing.T) {
	src := []byte(`package main
func Foo() {}
func Bar() {}
`)
	d := openTestDB(t)
	docID := mustInsertFile(t, d, "/a/main.go", string(src), "src")
	syms, _ := extractSymbols(src, "go")
	insertChunks(context.Background(), d, docID, "/a/main.go", src, syms, "go", "src")

	var before int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='chunk'`).Scan(&before)
	if before != 2 {
		t.Fatalf("setup: want 2 chunks, got %d", before)
	}

	// Re-index with only Foo remaining — Bar's chunk should be pruned.
	srcV2 := []byte(`package main
func Foo() {}
`)
	symsV2, _ := extractSymbols(srcV2, "go")
	insertChunks(context.Background(), d, docID, "/a/main.go", srcV2, symsV2, "go", "src")

	var after int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='chunk'`).Scan(&after)
	if after != 1 {
		t.Errorf("want 1 chunk after Bar removed, got %d", after)
	}
}

func TestIndexDir_ProducesChunks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main
func Hello() {}
func World() {}
`)
	d := openTestDB(t)
	IndexDir(context.Background(), d, dir, "src")

	var chunks int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='chunk'`).Scan(&chunks)
	if chunks < 2 {
		t.Errorf("want at least 2 chunks (Hello, World), got %d", chunks)
	}
}

func TestIndexDir_FileWithNoSymbolsHasNoChunks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", `# hello world`)
	d := openTestDB(t)
	IndexDir(context.Background(), d, dir, "src")

	var chunks int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='chunk'`).Scan(&chunks)
	if chunks != 0 {
		t.Errorf("markdown file should produce no chunks, got %d", chunks)
	}
}

// ---- helpers ----

func mustInsertFile(t *testing.T, d *sql.DB, path, content, source string) int64 {
	t.Helper()
	id, err := upsertDocument(context.Background(), d, path, "file", content, "", source, 0)
	if err != nil {
		t.Fatalf("insert file %q: %v", path, err)
	}
	return id
}
