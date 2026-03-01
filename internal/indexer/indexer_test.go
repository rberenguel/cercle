package indexer

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ruben/cercle/internal/db"
)

// ---- detectLang ----

func TestDetectLang(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.ts", "javascript"},
		{"README.md", ""},
		{"data.json", ""},
		{"config.yaml", ""},
		{"config.yml", ""},
		{"run.sh", ""},
		{"Cargo.toml", ""},
		{"unknown.xyz", ""},
	}
	for _, tc := range cases {
		got := detectLang(tc.file)
		if got != tc.want {
			t.Errorf("detectLang(%q) = %q, want %q", tc.file, got, tc.want)
		}
	}
}

// ---- isSupportedFile ----

func TestIsSupportedFile(t *testing.T) {
	supported := []string{
		"main.go", "app.py", "index.js", "Component.ts",
		"README.md", "notes.txt", "config.json",
		"config.yaml", "config.yml", "Cargo.toml", "deploy.sh",
	}
	for _, f := range supported {
		if !isSupportedFile(f) {
			t.Errorf("isSupportedFile(%q) = false, want true", f)
		}
	}

	unsupported := []string{
		"image.png", "photo.jpg", "binary.exe", "archive.zip", "data.parquet",
	}
	for _, f := range unsupported {
		if isSupportedFile(f) {
			t.Errorf("isSupportedFile(%q) = true, want false", f)
		}
	}
}

// ---- shouldSkipDir ----

func TestShouldSkipDir(t *testing.T) {
	skipped := []string{".git", "node_modules", "vendor", ".venv", "__pycache__", "dist", "build", ".claude", ".gemini"}
	for _, d := range skipped {
		if !shouldSkipDir(d) {
			t.Errorf("shouldSkipDir(%q) = false, want true", d)
		}
	}

	kept := []string{"src", "internal", "pkg", "lib", "test", "docs"}
	for _, d := range kept {
		if shouldSkipDir(d) {
			t.Errorf("shouldSkipDir(%q) = true, want false", d)
		}
	}
}

// ---- IndexDir end-to-end ----

func TestIndexDir_CountsFilesAndSymbols(t *testing.T) {
	dir := t.TempDir()

	// A Go file with two symbols.
	writeFile(t, dir, "main.go", `package main

func Hello() {}
func World() {}
`)
	// A JS file with a class and a method.
	writeFile(t, dir, "app.js", `class App {
  run() {}
}
`)
	// A markdown file (no symbols, FTS5 only).
	writeFile(t, dir, "README.md", `# hello`)

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "test-source")
	if err != nil {
		t.Fatalf("IndexDir: %v", err)
	}

	if result.Files != 3 {
		t.Errorf("want 3 files, got %d", result.Files)
	}
	// Go: Hello, World. JS: App (class), run (method).
	if result.Symbols < 4 {
		t.Errorf("want at least 4 symbols, got %d", result.Symbols)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestIndexDir_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.js", `function main() {}`)
	os.MkdirAll(filepath.Join(dir, "node_modules", "lib"), 0755)
	writeFile(t, filepath.Join(dir, "node_modules", "lib"), "index.js", `function skip() {}`)

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "src")
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 {
		t.Errorf("want 1 file (node_modules skipped), got %d", result.Files)
	}
}

func TestIndexDir_UpsertOnReindex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.go", `package main
func Foo() {}
`)
	d := openTestDB(t)

	r1, err := IndexDir(context.Background(), d, dir, "src")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Files != 1 {
		t.Fatalf("first index: want 1 file, got %d", r1.Files)
	}

	// Re-index without changing content — should not error.
	r2, err := IndexDir(context.Background(), d, dir, "src")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Files != 1 {
		t.Errorf("re-index: want 1 file, got %d", r2.Files)
	}
	if len(r2.Errors) > 0 {
		t.Errorf("re-index: unexpected errors: %v", r2.Errors)
	}

	// Verify only one document row exists.
	var count int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM documents WHERE kind='file'`).Scan(&count)
	if count != 1 {
		t.Errorf("want 1 document row after re-index, got %d", count)
	}
}

func TestIndexDir_ReindexInvalidatesEmbedding(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.go", `package main
func Foo() {}
`)
	d := openTestDB(t)
	IndexDir(context.Background(), d, dir, "src")

	// Manually insert a fake embedding for the indexed document.
	var docID int64
	d.QueryRowContext(context.Background(), `SELECT id FROM documents LIMIT 1`).Scan(&docID)
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO embeddings (doc_id, model, vector) VALUES (?, 'test-model', X'00000000')`, docID)
	if err != nil {
		t.Fatalf("insert embedding: %v", err)
	}

	// Re-index with changed content.
	writeFile(t, dir, "app.go", `package main
func Foo() {}
func Bar() {}
`)
	IndexDir(context.Background(), d, dir, "src")

	// Embedding must have been deleted by the trigger.
	var count int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count != 0 {
		t.Errorf("want 0 embeddings after content change (stale embedding should be invalidated), got %d", count)
	}
}

func TestIndexDir_ReindexSameContentKeepsEmbedding(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.go", `package main
func Foo() {}
`)
	d := openTestDB(t)
	IndexDir(context.Background(), d, dir, "src")

	var docID int64
	d.QueryRowContext(context.Background(), `SELECT id FROM documents LIMIT 1`).Scan(&docID)
	_, _ = d.ExecContext(context.Background(),
		`INSERT INTO embeddings (doc_id, model, vector) VALUES (?, 'test-model', X'00000000')`, docID)

	// Re-index with identical content — embedding must survive.
	IndexDir(context.Background(), d, dir, "src")

	var count int
	d.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM embeddings WHERE doc_id = ?`, docID).Scan(&count)
	if count != 1 {
		t.Errorf("want 1 embedding after no-op re-index (same content), got %d", count)
	}
}

func TestIndexDir_SourceTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", `package main`)

	d := openTestDB(t)
	IndexDir(context.Background(), d, dir, "my-project")

	var source string
	d.QueryRowContext(context.Background(), `SELECT source FROM documents LIMIT 1`).Scan(&source)
	if source != "my-project" {
		t.Errorf("want source=my-project, got %q", source)
	}
}

// ---- helpers ----

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
