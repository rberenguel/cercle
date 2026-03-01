package search

import (
	"context"
	"strings"
	"testing"
)

func TestReadSymbol_ReturnsFullBody(t *testing.T) {
	d := openTestDB(t)
	docID := insertDoc(t, d, "/src/foo.go", "package main\nfunc Foo() {\n\treturn\n}", "src")
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO symbols (doc_id, kind, name, signature, start_line, end_line) VALUES (?, 'function', 'Foo', 'func Foo()', 2, 4)`,
		docID)
	if err != nil {
		t.Fatalf("insert symbol: %v", err)
	}
	// Insert chunk document matching the path convention.
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, source, parent_id) VALUES (?, 'chunk', ?, 'src', ?)`,
		"/src/foo.go::Foo@2", "func Foo() {\n\treturn\n}", docID)
	if err != nil {
		t.Fatalf("insert chunk: %v", err)
	}

	result, err := ReadSymbol(context.Background(), d, "foo.go", "Foo", "/src")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Name != "Foo" {
		t.Errorf("want name=Foo, got %q", result.Name)
	}
	if !strings.Contains(result.Content, "func Foo()") {
		t.Errorf("want content to contain function body, got %q", result.Content)
	}
}

func TestReadSymbol_NotFound(t *testing.T) {
	d := openTestDB(t)
	result, err := ReadSymbol(context.Background(), d, "missing.go", "NoSuch", "/src")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for missing symbol, got %+v", result)
	}
}

func TestReadContext_ReturnsWindow(t *testing.T) {
	d := openTestDB(t)
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	insertDoc(t, d, "/src/bar.go", content, "src")

	result, err := ReadContext(context.Background(), d, "bar.go", 5, 2, "/src")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.StartLine != 3 || result.EndLine != 7 {
		t.Errorf("want lines 3–7, got %d–%d", result.StartLine, result.EndLine)
	}
	if !strings.Contains(result.Content, "line5") {
		t.Errorf("expected line5 in context, got %q", result.Content)
	}
}

func TestReadContext_ClampsToBounds(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/src/bar.go", "line1\nline2\nline3", "src")

	result, err := ReadContext(context.Background(), d, "bar.go", 1, 10, "/src")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.StartLine != 1 || result.EndLine != 3 {
		t.Errorf("want lines 1–3 (clamped), got %d–%d", result.StartLine, result.EndLine)
	}
}

func TestReadContext_NotFound(t *testing.T) {
	d := openTestDB(t)
	result, err := ReadContext(context.Background(), d, "missing.go", 5, 5, "/src")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for missing file, got %+v", result)
	}
}
