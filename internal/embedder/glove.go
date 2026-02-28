// Package embedder loads pre-trained word vectors (GloVe or FastText text
// format) and produces document embeddings by averaging token vectors.
// No network calls, no external services — the model file is the only dependency.
package embedder

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// Embedder holds the in-memory word vector table.
type Embedder struct {
	vectors map[string][]float32
	dim     int
}

// Dim returns the vector dimensionality.
func (e *Embedder) Dim() int { return e.dim }

// Len returns the number of words in the vocabulary.
func (e *Embedder) Len() int { return len(e.vectors) }

// Load reads a GloVe or FastText text-format vector file from an io.Reader.
func Load(r io.Reader) (*Embedder, error) {
	return load(r)
}

// LoadFile reads a GloVe or FastText text-format vector file.
// Format per line: "<word> <f1> <f2> ... <fN>"
// FastText files may have a header line "<vocab_size> <dim>" which is skipped.
func LoadFile(path string) (*Embedder, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vectors: %w", err)
	}
	defer f.Close()
	return load(f)
}

func load(r io.Reader) (*Embedder, error) {
	scanner := bufio.NewScanner(r)
	// Increase buffer for long lines (300d lines can exceed default 64KB).
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	e := &Embedder{vectors: make(map[string][]float32)}
	lineNo := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNo++

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Skip FastText header (two integers: vocab_size dim).
		if lineNo == 1 {
			if _, err := strconv.Atoi(parts[0]); err == nil {
				continue
			}
		}

		word := parts[0]
		vals := parts[1:]
		vec := make([]float32, len(vals))
		for i, v := range vals {
			f, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return nil, fmt.Errorf("line %d: parse float: %w", lineNo, err)
			}
			vec[i] = float32(f)
		}

		if e.dim == 0 {
			e.dim = len(vec)
		} else if len(vec) != e.dim {
			log.Printf("embedder: line %d: dim mismatch (%d vs %d), skipping", lineNo, len(vec), e.dim)
			continue
		}

		e.vectors[strings.ToLower(word)] = vec
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	if e.dim == 0 {
		return nil, fmt.Errorf("no vectors loaded from file")
	}

	log.Printf("embedder: loaded %d vectors (dim=%d)", len(e.vectors), e.dim)
	return e, nil
}

// Embed produces a document-level embedding by tokenising text and averaging
// the vectors of known tokens. Returns nil if no tokens matched.
func (e *Embedder) Embed(text string) []float32 {
	tokens := tokenise(text)
	result := make([]float32, e.dim)
	count := 0

	for _, tok := range tokens {
		vec, ok := e.vectors[tok]
		if !ok {
			continue
		}
		for i, v := range vec {
			result[i] += v
		}
		count++
	}

	if count == 0 {
		return nil
	}

	// Normalise to unit vector.
	var norm float64
	for _, v := range result {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range result {
			result[i] = float32(float64(result[i]) / norm)
		}
	}

	return result
}

// tokenise splits text into lowercase tokens, handling:
// - natural word boundaries (spaces, punctuation)
// - camelCase:  "IndexDir"    → ["index", "dir"]
// - snake_case: "embed_store" → ["embed", "store"]
// - digits are stripped (rarely useful in GloVe vocab)
func tokenise(text string) []string {
	// First pass: split camelCase.
	text = splitCamel(text)
	// Lowercase everything.
	text = strings.ToLower(text)
	// Replace non-alpha with space.
	var b strings.Builder
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}

	parts := strings.Fields(b.String())
	tokens := parts[:0]
	for _, p := range parts {
		if len(p) >= 3 { // skip very short tokens
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// splitCamel inserts spaces at camelCase and PascalCase word boundaries.
//
//	"IndexDir"      → "Index Dir"
//	"HTMLParser"    → "HTML Parser"   (uppercase run → title case)
//	"GetHTTPClient" → "Get HTTP Client"
func splitCamel(s string) string {
	var b strings.Builder
	runes := []rune(s)
	n := len(runes)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' {
				// foo|Bar
				b.WriteByte(' ')
			} else if prev >= 'A' && prev <= 'Z' && i+1 < n {
				// HTML|Parser — end of an uppercase run before a lowercase letter
				if runes[i+1] >= 'a' && runes[i+1] <= 'z' {
					b.WriteByte(' ')
				}
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
