package search

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSemantic_NilEmbedder(t *testing.T) {
	d := openTestDB(t)
	_, err := Semantic(context.Background(), d, nil, "player", "", 10, 0)
	if err == nil {
		t.Error("expected error for nil embedder, got nil")
	}
}

func TestSemantic_EmptyResultsIsArray(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)
	// No documents indexed, so no embeddings exist.
	results, err := Semantic(context.Background(), d, emb, "player game", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(results)
	if string(b) == "null" {
		t.Error("empty results must marshal to [] not null")
	}
}

func TestSemantic_FindsMatch(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	idA := insertDoc(t, d, "/a/player.js", "player game combat", "src")
	idB := insertDoc(t, d, "/b/physics.js", "physics simulation", "src")

	if err := EmbedAndStore(context.Background(), d, emb, idA, "player game"); err != nil {
		t.Fatalf("embed A: %v", err)
	}
	if err := EmbedAndStore(context.Background(), d, emb, idB, "physics"); err != nil {
		t.Fatalf("embed B: %v", err)
	}

	results, err := Semantic(context.Background(), d, emb, "player game", "src", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	// /a/player.js should rank first (highest similarity to "player game").
	if results[0].Path != "/a/player.js" {
		t.Errorf("want /a/player.js first, got %q (similarity=%.3f)", results[0].Path, results[0].Similarity)
	}
}

func TestSemantic_MinSimilarityFiltersNoise(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	// "player game" → [0.707, 0.707, 0, 0] after normalisation
	// "enemy physics" → [0, 0, 0.707, 0.707] after normalisation
	// cosine("player game", "enemy physics") = 0 → filtered by any threshold > 0
	idA := insertDoc(t, d, "/a/a.js", "player game", "src")
	idB := insertDoc(t, d, "/b/b.js", "enemy physics", "src")
	if err := EmbedAndStore(context.Background(), d, emb, idA, "player game"); err != nil {
		t.Fatal(err)
	}
	if err := EmbedAndStore(context.Background(), d, emb, idB, "enemy physics"); err != nil {
		t.Fatal(err)
	}

	results, err := Semantic(context.Background(), d, emb, "player game", "src", 10, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Similarity < 0.5 {
			t.Errorf("result %q has similarity %.3f, below min_similarity=0.5", r.Path, r.Similarity)
		}
	}
	// "enemy physics" should be excluded
	for _, r := range results {
		if r.Path == "/b/b.js" {
			t.Errorf("/b/b.js should be filtered out (similarity=%.3f)", r.Similarity)
		}
	}
}

func TestSemantic_AllUnknownTokens(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t) // vocab: player, game, enemy, physics only

	_, err := Semantic(context.Background(), d, emb, "xyzzy foobar quux", "", 10, 0)
	if err == nil {
		t.Error("expected error when query has no tokens in vocab, got nil")
	}
}

func TestSemantic_LimitResults(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	for i, path := range []string{"/a.js", "/b.js", "/c.js"} {
		id := insertDoc(t, d, path, "player game", "src")
		_ = i
		if err := EmbedAndStore(context.Background(), d, emb, id, "player game"); err != nil {
			t.Fatal(err)
		}
	}

	results, err := Semantic(context.Background(), d, emb, "player game", "src", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 2 {
		t.Errorf("want at most 2 results (limit), got %d", len(results))
	}
}

func TestSemantic_SourceFilter(t *testing.T) {
	d := openTestDB(t)
	emb := testEmbedder(t)

	idA := insertDoc(t, d, "/a/a.js", "player game", "project-a")
	idB := insertDoc(t, d, "/b/b.js", "player game", "project-b")
	if err := EmbedAndStore(context.Background(), d, emb, idA, "player game"); err != nil {
		t.Fatal(err)
	}
	if err := EmbedAndStore(context.Background(), d, emb, idB, "player game"); err != nil {
		t.Fatal(err)
	}

	results, err := Semantic(context.Background(), d, emb, "player game", "project-a", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Source != "project-a" {
			t.Errorf("result %q has source %q, want project-a", r.Path, r.Source)
		}
	}
}
