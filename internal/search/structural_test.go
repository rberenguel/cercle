package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func addSymbol(t *testing.T, d *sql.DB, docID int64, kind, name string) {
	t.Helper()
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO symbols (doc_id, kind, name, start_line, end_line) VALUES (?, ?, ?, 1, 1)`,
		docID, kind, name)
	if err != nil {
		t.Fatalf("insert symbol %q: %v", name, err)
	}
}

func TestStructural_EmptyResultsIsArray(t *testing.T) {
	d := openTestDB(t)
	results, err := Structural(context.Background(), d, "NoSuchSymbol", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(results)
	if string(b) == "null" {
		t.Error("empty results must marshal to [] not null")
	}
}

func TestStructural_FindsSymbol(t *testing.T) {
	d := openTestDB(t)
	docID := insertDoc(t, d, "/a/b.go", "func Foo() {}", "src")
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO symbols (doc_id, kind, name, signature, start_line, end_line) VALUES (?, 'function', 'Foo', 'func Foo()', 1, 1)`,
		docID)
	if err != nil {
		t.Fatalf("insert symbol: %v", err)
	}

	results, err := Structural(context.Background(), d, "Foo", "src", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	if results[0].Name != "Foo" {
		t.Errorf("want name=Foo, got %q", results[0].Name)
	}
	if results[0].Kind != "function" {
		t.Errorf("want kind=function, got %q", results[0].Kind)
	}
}

func TestStructural_PrefixMatch(t *testing.T) {
	d := openTestDB(t)
	docID := insertDoc(t, d, "/a/b.go", "...", "src")
	addSymbol(t, d, docID, "function", "FooBar")
	addSymbol(t, d, docID, "function", "FooBaz")
	addSymbol(t, d, docID, "function", "Qux")

	results, err := Structural(context.Background(), d, "Foo", "src", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results (FooBar, FooBaz), got %d: %+v", len(results), results)
	}
}

func TestStructural_GlobalSearch(t *testing.T) {
	d := openTestDB(t)
	idA := insertDoc(t, d, "/a/a.go", "...", "project-a")
	idB := insertDoc(t, d, "/b/b.go", "...", "project-b")
	addSymbol(t, d, idA, "function", "SharedFn")
	addSymbol(t, d, idB, "function", "SharedFn")

	// No source filter — should find both.
	results, err := Structural(context.Background(), d, "SharedFn", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("global search: want 2 results, got %d", len(results))
	}
}

func TestStructural_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	idA := insertDoc(t, d, "/a/a.go", "...", "project-a")
	idB := insertDoc(t, d, "/b/b.go", "...", "project-b")
	addSymbol(t, d, idA, "function", "Shared")
	addSymbol(t, d, idB, "function", "Shared")

	results, err := Structural(context.Background(), d, "Shared", "project-a", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Path != "/a/a.go" {
		t.Errorf("want /a/a.go, got %q", results[0].Path)
	}
}

func TestStructural_PreviewFromChunk(t *testing.T) {
	d := openTestDB(t)
	body := "func Foo() {\n\tline1\n\tline2\n\tline3\n\tline4\n\tline5\n\tline6\n}"
	docID := insertDoc(t, d, "/a.go", body, "src")
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO symbols (doc_id, kind, name, signature, start_line, end_line) VALUES (?, 'function', 'Foo', 'func Foo()', 1, 8)`,
		docID)
	if err != nil {
		t.Fatalf("insert symbol: %v", err)
	}
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, lang, source, parent_id) VALUES ('/a.go::Foo@1', 'chunk', ?, 'go', 'src', ?)`,
		body, docID)
	if err != nil {
		t.Fatalf("insert chunk: %v", err)
	}

	results, err := Structural(context.Background(), d, "Foo", "src", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Preview == "" {
		t.Error("want non-empty Preview from chunk")
	}
	lines := strings.Split(results[0].Preview, "\n")
	if len(lines) > 20 {
		t.Errorf("want at most 20 preview lines, got %d", len(lines))
	}
}

func TestStructural_RelativizesPathsToSource(t *testing.T) {
	d := openTestDB(t)
	docID := insertDoc(t, d, "/proj/internal/a.go", "...", "/proj")
	addSymbol(t, d, docID, "function", "MyFn")

	results, err := Structural(context.Background(), d, "MyFn", "/proj", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Path != "internal/a.go" {
		t.Errorf("want relative path internal/a.go, got %q", results[0].Path)
	}
}
