package search

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/ruben/cercle/internal/embedder"
)

// SemanticResult is a single vector-similarity hit.
type SemanticResult struct {
	ID         int64   `json:"id"`
	Path       string  `json:"path"`
	Source     string  `json:"source"`
	Kind       string  `json:"kind"`
	Similarity float64 `json:"similarity"`
}

// Semantic embeds the query using the provided Embedder and returns nearest
// neighbours by cosine similarity.
// If source is non-empty, only documents with that source tag are considered.
// minSimilarity filters out results below the threshold (0 = no filter).
func Semantic(ctx context.Context, db *sql.DB, emb *embedder.Embedder, query, source string, limit int, minSimilarity float64) ([]SemanticResult, error) {
	if emb == nil {
		return nil, fmt.Errorf("semantic search unavailable: no word vectors loaded (run the download script)")
	}
	if limit <= 0 {
		limit = 10
	}

	qVec := emb.Embed(query)
	if qVec == nil {
		return nil, fmt.Errorf("no known tokens in query %q — try different words", query)
	}

	var (
		rows *sql.Rows
		err  error
	)
	if source != "" {
		rows, err = db.QueryContext(ctx, `
			SELECT e.doc_id, d.path, COALESCE(d.source,''), d.kind, e.vector
			FROM embeddings e
			JOIN documents d ON d.id = e.doc_id
			WHERE e.model = ? AND d.source = ?
		`, ModelID(emb), source)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT e.doc_id, d.path, COALESCE(d.source,''), d.kind, e.vector
			FROM embeddings e
			JOIN documents d ON d.id = e.doc_id
			WHERE e.model = ?
		`, ModelID(emb))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		SemanticResult
		vec []float32
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		var blob []byte
		if err := rows.Scan(&c.ID, &c.Path, &c.Source, &c.Kind, &blob); err != nil {
			return nil, err
		}
		c.vec = blobToFloat32(blob)
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type scored struct {
		SemanticResult
		sim float64
	}
	var hits []scored
	for _, c := range candidates {
		sim := cosineSimilarity(qVec, c.vec)
		if minSimilarity > 0 && sim < minSimilarity {
			continue
		}
		hits = append(hits, scored{c.SemanticResult, sim})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })

	if len(hits) > limit {
		hits = hits[:limit]
	}
	results := make([]SemanticResult, len(hits))
	for i, h := range hits {
		results[i] = h.SemanticResult
		results[i].Similarity = h.sim
	}
	return results, nil
}

// EmbedAndStore generates an embedding for docID and stores it.
func EmbedAndStore(ctx context.Context, db *sql.DB, emb *embedder.Embedder, docID int64, text string) error {
	if emb == nil {
		return nil // silently skip; daemon was started without vectors
	}
	vec := emb.Embed(text)
	if vec == nil {
		return nil // no known tokens; skip rather than store a zero vector
	}
	blob := float32ToBlob(vec)
	_, err := db.ExecContext(ctx, `
		INSERT INTO embeddings (doc_id, model, vector) VALUES (?, ?, ?)
		ON CONFLICT(doc_id, model) DO UPDATE SET vector = excluded.vector
	`, docID, ModelID(emb), blob)
	return err
}

// ModelID returns a stable string identifier for the loaded model based on
// its dimensionality. This is stored in the embeddings table so queries only
// compare vectors from the same model.
func ModelID(emb *embedder.Embedder) string {
	return fmt.Sprintf("glove-local-dim%d", emb.Dim())
}

// PendingCount returns the number of documents without embeddings.
func PendingCount(ctx context.Context, db *sql.DB, emb *embedder.Embedder) (int, error) {
	if emb == nil {
		return 0, nil
	}
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		WHERE NOT EXISTS (
			SELECT 1 FROM embeddings e WHERE e.doc_id = d.id AND e.model = ?
		)
		AND NOT (
			d.kind = 'file' AND EXISTS (
				SELECT 1 FROM documents c WHERE c.parent_id = d.id
			)
		)
	`, ModelID(emb)).Scan(&n)
	return n, err
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func blobToFloat32(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
