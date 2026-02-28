package search

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLexical_EmptyResultsIsArray(t *testing.T) {
	d := openTestDB(t)
	results, err := Lexical(context.Background(), d, "zzznomatch", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(results)
	if string(b) == "null" {
		t.Error("empty results must marshal to [] not null")
	}
}

func TestLexical_FindsMatch(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/b.go", "hello world function", "src")

	results, err := Lexical(context.Background(), d, "hello", "src", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	if results[0].Path != "/a/b.go" {
		t.Errorf("want /a/b.go, got %q", results[0].Path)
	}
}

func TestLexical_SnippetUsesStarMarkers(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/b.go", "hello world function", "src")

	results, err := Lexical(context.Background(), d, "hello", "src", 10)
	if err != nil || len(results) == 0 {
		t.Fatal("no results")
	}
	snippet := results[0].Snippet
	if strings.Contains(snippet, "<b>") || strings.Contains(snippet, "\u003c") {
		t.Errorf("snippet contains raw HTML: %q", snippet)
	}
	if !strings.Contains(snippet, "**") {
		t.Errorf("snippet missing ** markers: %q", snippet)
	}
}

func TestLexical_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/b.go", "hello world", "project-a")
	insertDoc(t, d, "/c/d.go", "hello world", "project-b")

	results, err := Lexical(context.Background(), d, "hello", "project-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Path != "/a/b.go" {
		t.Errorf("want /a/b.go, got %q", results[0].Path)
	}
}

func TestLexical_GlobalSearch(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "hello world", "project-a")
	insertDoc(t, d, "/b.go", "hello world", "project-b")

	// No source filter — should find both.
	results, err := Lexical(context.Background(), d, "hello", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("global search: want 2 results, got %d", len(results))
	}
}

func TestLexical_PorterStemming(t *testing.T) {
	d := openTestDB(t)
	// Index "running" — porter stemmer should match query "run".
	insertDoc(t, d, "/a.go", "the process is running smoothly", "src")

	results, err := Lexical(context.Background(), d, "run", "src", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("porter stemming: query 'run' should match document containing 'running'")
	}
}

func TestLexical_RankedOrder(t *testing.T) {
	d := openTestDB(t)
	// /a.go mentions "error" once; /b.go mentions it three times — /b.go should rank higher.
	insertDoc(t, d, "/a.go", "there was an error here", "src")
	insertDoc(t, d, "/b.go", "error error error everything is broken", "src")

	results, err := Lexical(context.Background(), d, "error", "src", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatal("expected at least 2 results")
	}
	if results[0].Path != "/b.go" {
		t.Errorf("want /b.go (more matches) first, got %q", results[0].Path)
	}
}

func TestLexical_Limit(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "the quick brown fox", "src")
	insertDoc(t, d, "/b.go", "the lazy dog jumped", "src")
	insertDoc(t, d, "/c.go", "the cat sat down", "src")

	results, err := Lexical(context.Background(), d, "the", "src", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 2 {
		t.Errorf("want at most 2 results, got %d", len(results))
	}
}
