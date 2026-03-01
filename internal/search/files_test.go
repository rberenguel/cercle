package search

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFiles_EmptyResultsIsArray(t *testing.T) {
	d := openTestDB(t)
	files, err := Files(context.Background(), d, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(files)
	if string(b) == "null" {
		t.Error("empty results must marshal to [] not null")
	}
}

func TestFiles_ReturnsIndexedFiles(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "...", "src")
	insertDoc(t, d, "/b.go", "...", "src")

	files, err := Files(context.Background(), d, "src", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("want 2 files, got %d: %v", len(files), files)
	}
}

func TestFiles_ExcludesSummaries(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "...", "src")
	_, _ = d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, source) VALUES ('summary:1', 'summary', 'text', 'src')`)

	files, err := Files(context.Background(), d, "src", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("want 1 file (summaries excluded), got %d: %v", len(files), files)
	}
}

func TestFiles_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "...", "project-a")
	insertDoc(t, d, "/b.go", "...", "project-b")

	files, err := Files(context.Background(), d, "project-a", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "/a.go" {
		t.Errorf("want [/a.go], got %v", files)
	}
}

func TestFiles_GlobalSearch(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "...", "project-a")
	insertDoc(t, d, "/b.go", "...", "project-b")

	files, err := Files(context.Background(), d, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("global search: want 2 files, got %d", len(files))
	}
}

func TestFiles_SortedAlphabetically(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/z.go", "...", "src")
	insertDoc(t, d, "/a.go", "...", "src")
	insertDoc(t, d, "/m.go", "...", "src")

	files, err := Files(context.Background(), d, "src", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %d", len(files))
	}
	if files[0] != "/a.go" || files[1] != "/m.go" || files[2] != "/z.go" {
		t.Errorf("want alphabetical order, got %v", files)
	}
}

func TestFiles_Limit(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "...", "src")
	insertDoc(t, d, "/b.go", "...", "src")
	insertDoc(t, d, "/c.go", "...", "src")

	files, err := Files(context.Background(), d, "src", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("want 2 files (limit), got %d", len(files))
	}
}

func TestFiles_GlobFilter(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/project/internal/search/lexical.go", "...", "src")
	insertDoc(t, d, "/project/internal/search/semantic.go", "...", "src")
	insertDoc(t, d, "/project/cmd/main.go", "...", "src")
	insertDoc(t, d, "/project/README.md", "...", "src")

	files, err := Files(context.Background(), d, "src", "*/internal/search/*.go", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("want 2 files matching glob, got %d: %v", len(files), files)
	}
}
