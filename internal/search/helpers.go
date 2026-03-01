package search

import "strings"

// relativizePath strips the source namespace prefix from a path so results
// are relative to the indexed root. Handles chunk paths ({file}::{sym}@{line}).
// Returns the path unchanged when source is empty or is not a prefix.
func relativizePath(path, source string) string {
	if source == "" {
		return path
	}
	prefix := strings.TrimRight(source, "/") + "/"
	if idx := strings.Index(path, "::"); idx != -1 {
		// Chunk path — only relativize the file portion before "::".
		return strings.TrimPrefix(path[:idx], prefix) + path[idx:]
	}
	return strings.TrimPrefix(path, prefix)
}

// firstLines returns up to n lines of s joined by newlines.
func firstLines(s string, n int) string {
	if s == "" {
		return ""
	}
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
