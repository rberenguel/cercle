package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ---- unit tests (no temp files) ----

func TestIgnoreList_EmptyIgnoresNothing(t *testing.T) {
	var il IgnoreList
	if il.ShouldIgnore("foo/bar.go", false) {
		t.Error("empty IgnoreList should not ignore anything")
	}
	if il.ShouldIgnore("foo", true) {
		t.Error("empty IgnoreList should not ignore directories")
	}
}

func TestIgnoreList_SimpleBasenamePattern(t *testing.T) {
	il := &IgnoreList{}
	il.patterns = []ignorePattern{{pattern: "*.db"}}

	if !il.ShouldIgnore("foo/bar.db", false) {
		t.Error("*.db should match foo/bar.db")
	}
	if il.ShouldIgnore("foo/bar.go", false) {
		t.Error("*.db should not match foo/bar.go")
	}
}

func TestIgnoreList_AnchoredPattern(t *testing.T) {
	il := &IgnoreList{}
	il.patterns = []ignorePattern{{anchored: true, pattern: "eval/results"}}

	if !il.ShouldIgnore("eval/results", false) {
		t.Error("eval/results should match eval/results")
	}
	if il.ShouldIgnore("other/eval/results", false) {
		t.Error("eval/results should not match other/eval/results")
	}
}

func TestIgnoreList_DirOnlyPattern(t *testing.T) {
	il := &IgnoreList{}
	il.patterns = []ignorePattern{{dirOnly: true, pattern: "bin"}}

	if !il.ShouldIgnore("bin", true) {
		t.Error("bin/ should match directory bin")
	}
	if il.ShouldIgnore("bin", false) {
		t.Error("bin/ should not match file bin")
	}
}

func TestIgnoreList_CommentAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	ignPath := filepath.Join(dir, ".cercleignore")
	os.WriteFile(ignPath, []byte(`
# this is a comment
*.db

# another comment
`), 0644)

	il := &IgnoreList{}
	if err := il.LoadIgnoreFile(ignPath); err != nil {
		t.Fatalf("LoadIgnoreFile: %v", err)
	}
	if len(il.patterns) != 1 {
		t.Errorf("want 1 pattern, got %d", len(il.patterns))
	}
	if il.patterns[0].pattern != "*.db" {
		t.Errorf("want pattern *.db, got %q", il.patterns[0].pattern)
	}
}

// ---- integration tests ----

func TestIndexDir_CercleignoreSkipsFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main`)
	writeFile(t, dir, "secret.db", `binary data`)
	writeFile(t, dir, ".cercleignore", "*.db\n")

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "test")
	if err != nil {
		t.Fatalf("IndexDir: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("want 1 file (secret.db excluded), got %d", result.Files)
	}
}

func TestIndexDir_CercleignoreSkipsDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main`)
	os.MkdirAll(filepath.Join(dir, "private"), 0755)
	writeFile(t, filepath.Join(dir, "private"), "data.go", `package private`)
	writeFile(t, dir, ".cercleignore", "private/\n")

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "test")
	if err != nil {
		t.Fatalf("IndexDir: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("want 1 file (private/ dir excluded), got %d", result.Files)
	}
}

func TestIndexDir_GitignoreIsRespected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main`)
	writeFile(t, dir, "notes.txt", `some notes`)
	writeFile(t, dir, ".gitignore", "*.txt\n")

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "test")
	if err != nil {
		t.Fatalf("IndexDir: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("want 1 file (notes.txt excluded via .gitignore), got %d", result.Files)
	}
}

func TestIndexDir_CercleignoreAndGitignoreCombine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main`)
	writeFile(t, dir, "notes.txt", `some notes`)
	writeFile(t, dir, "secret.db", `binary`)
	writeFile(t, dir, ".gitignore", "*.txt\n")
	writeFile(t, dir, ".cercleignore", "*.db\n")

	d := openTestDB(t)
	result, err := IndexDir(context.Background(), d, dir, "test")
	if err != nil {
		t.Fatalf("IndexDir: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("want 1 file (txt+db excluded), got %d", result.Files)
	}
}

func TestLoadIgnoreFile_MissingFileIsNotAnError(t *testing.T) {
	il := &IgnoreList{}
	err := il.LoadIgnoreFile("/nonexistent/path/.cercleignore")
	if err != nil {
		t.Errorf("missing file should not return error, got: %v", err)
	}
}
