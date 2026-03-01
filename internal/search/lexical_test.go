package search

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLexical_EmptyResultsIsArray(t *testing.T) {
	d := openTestDB(t)
	results, err := Lexical(context.Background(), d, "zzznomatch", "", 10, false)
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

	results, err := Lexical(context.Background(), d, "hello", "src", 10, false)
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

func TestLexical_SnippetIsPlainText(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/b.go", "hello world function", "src")

	results, err := Lexical(context.Background(), d, "hello", "src", 10, false)
	if err != nil || len(results) == 0 {
		t.Fatal("no results")
	}
	snippet := results[0].Snippet
	if strings.Contains(snippet, "<b>") || strings.Contains(snippet, "\u003c") {
		t.Errorf("snippet contains raw HTML: %q", snippet)
	}
	if strings.Contains(snippet, "**") {
		t.Errorf("snippet must be plain text, got bold markers: %q", snippet)
	}
	if snippet == "" {
		t.Error("snippet must not be empty")
	}
}

func TestLexical_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a/b.go", "hello world", "project-a")
	insertDoc(t, d, "/c/d.go", "hello world", "project-b")

	results, err := Lexical(context.Background(), d, "hello", "project-a", 10, false)
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
	results, err := Lexical(context.Background(), d, "hello", "", 10, false)
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

	results, err := Lexical(context.Background(), d, "run", "src", 10, false)
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

	results, err := Lexical(context.Background(), d, "error", "src", 10, false)
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

	results, err := Lexical(context.Background(), d, "the", "src", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 2 {
		t.Errorf("want at most 2 results, got %d", len(results))
	}
}

func TestLexical_SanitizesDots(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/main.go", "call os Exit to terminate", "src")

	// "os.Exit" would break FTS5 with raw=true; sanitized it becomes "os Exit".
	results, err := Lexical(context.Background(), d, "os.Exit", "src", 10, false)
	if err != nil {
		t.Fatalf("sanitized query should not error: %v", err)
	}
	if len(results) == 0 {
		t.Error("want results for sanitized 'os Exit', got none")
	}
}

func TestLexical_SanitizesHyphens(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/doc.md", "command line tool for parsing", "src")

	// "command-line" has FTS5 NOT semantics; sanitized it becomes "command line".
	results, err := Lexical(context.Background(), d, "command-line", "src", 10, false)
	if err != nil {
		t.Fatalf("sanitized query should not error: %v", err)
	}
	if len(results) == 0 {
		t.Error("want results for sanitized 'command line', got none")
	}
}

func TestLexical_RawPassesThroughFTS5Syntax(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/a.go", "hello world foo", "src")
	insertDoc(t, d, "/b.go", "hello bar baz", "src")

	// raw=true: FTS5 OR syntax should work.
	results, err := Lexical(context.Background(), d, "foo OR bar", "src", 10, true)
	if err != nil {
		t.Fatalf("raw FTS5 OR query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results for 'foo OR bar', got %d", len(results))
	}
}

func TestSanitizeFTSQuery(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"os.Exit", "os Exit"},
		{"command-line", "command line"},
		{"reset*", "reset"},
		{`"exact phrase"`, "exact phrase"},
		{"foo.bar.baz", "foo bar baz"},
		{"  multiple   spaces  ", "multiple spaces"},
		{"___underscores___", "___underscores___"},
		{"127.0.0.1:7770", "127 0 0 1 7770"},
	}
	for _, c := range cases {
		got := SanitizeFTSQuery(c.input)
		if got != c.want {
			t.Errorf("SanitizeFTSQuery(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestLexical_DeduplicatesChunkAndParent(t *testing.T) {
	d := openTestDB(t)
	body := `func Hello() string { return "hello" }`
	parentID := insertDoc(t, d, "/a.go", body, "src")
	// Insert a matching chunk document (as the indexer would).
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO documents (path, kind, content, lang, source, parent_id)
		 VALUES ('/a.go::Hello@1', 'chunk', ?, 'go', 'src', ?)`,
		body, parentID)
	if err != nil {
		t.Fatalf("insert chunk: %v", err)
	}

	results, err := Lexical(context.Background(), d, "Hello", "src", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	// Chunk and file both match, but only the chunk should be returned.
	if len(results) != 1 {
		t.Errorf("want 1 result (chunk only, parent suppressed), got %d", len(results))
	}
	if len(results) > 0 && !strings.Contains(results[0].Path, "::") {
		t.Errorf("want chunk path (contains ::), got %q", results[0].Path)
	}
}

func TestLexical_RelativizesPathsToSource(t *testing.T) {
	d := openTestDB(t)
	insertDoc(t, d, "/proj/internal/a.go", "unique_token_xyz", "/proj")

	results, err := Lexical(context.Background(), d, "unique_token_xyz", "/proj", 10, false)
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
