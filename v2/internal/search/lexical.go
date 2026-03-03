package search

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"cercle-v2/internal/models"
)

// SearchLexical uses `rg` to find matches, determines the enclosing chunk,
// and returns the deduplicated chunk bodies.
func SearchLexical(workspacePath, query string) ([]string, error) {
	cmd := exec.Command("rg", "-n", query)
	cmd.Dir = workspacePath

	var out bytes.Buffer
	cmd.Stdout = &out
	// Ignore stderr

	if err := cmd.Run(); err != nil {
		// If rg fails (e.g. no matches), return empty
		return nil, nil
	}

	lines := strings.Split(out.String(), "\n")

	// Map to keep track of already added chunks to avoid duplicates
	// Key format: "filepath:start-end"
	seenChunks := make(map[string]bool)
	var chunkBodies []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		relPath := parts[0]
		lineNumStr := parts[1]
		lineNum, err := strconv.Atoi(lineNumStr)
		if err != nil {
			continue
		}

		// Find the chunk for this file and line number
		fileChunks, err := loadChunksForFile(workspacePath, relPath)
		if err != nil || len(fileChunks) == 0 {
			continue
		}

		var matchedChunk *models.Chunk
		for _, chunk := range fileChunks {
			if lineNum >= chunk.Start && lineNum <= chunk.End {
				matchedChunk = &chunk
				break
			}
		}

		if matchedChunk != nil {
			chunkKey := fmt.Sprintf("%s:%d-%d", matchedChunk.File, matchedChunk.Start, matchedChunk.End)
			if !seenChunks[chunkKey] {
				seenChunks[chunkKey] = true
				body, err := ReadChunkBody(workspacePath, *matchedChunk)
				if err == nil {
					header := fmt.Sprintf("// File: %s (L%d-L%d)\n", matchedChunk.File, matchedChunk.Start, matchedChunk.End)
					chunkBodies = append(chunkBodies, header+body)
				}
			}
		}
	}

	return chunkBodies, nil
}

// ReadChunkBody reads the physical lines of code for a given chunk
func ReadChunkBody(workspacePath string, chunk models.Chunk) (string, error) {
	absPath := filepath.Join(workspacePath, chunk.File)
	file, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 1

	for scanner.Scan() {
		if currentLine > chunk.End {
			break
		}
		if currentLine >= chunk.Start {
			sb.WriteString(scanner.Text() + "\n")
		}
		currentLine++
	}

	return sb.String(), scanner.Err()
}
