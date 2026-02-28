package embedder

import (
	"math"
	"strings"
	"testing"
)

// ---- splitCamel ----

func TestSplitCamel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"IndexDir", "Index Dir"},
		{"HTMLParser", "HTML Parser"},
		{"GetHTTPClient", "Get HTTP Client"},
		{"NewHTTPServer", "New HTTP Server"},
		{"lowercase", "lowercase"},
		{"alreadySplit", "already Split"},
		{"FooBarBaz", "Foo Bar Baz"},
		{"camelCaseWord", "camel Case Word"},
		{"", ""},
	}
	for _, tc := range cases {
		got := splitCamel(tc.in)
		if got != tc.want {
			t.Errorf("splitCamel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---- tokenise ----

func TestTokenise(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{
			in:   "IndexDir",
			want: []string{"index", "dir"},
		},
		{
			in:   "embed_and_store",
			want: []string{"embed", "and", "store"},
		},
		{
			in:   "hello world",
			want: []string{"hello", "world"},
		},
		{
			// Short tokens (< 3 chars) must be dropped.
			in:   "a bb ccc dddd",
			want: []string{"ccc", "dddd"},
		},
		{
			// Digits stripped.
			in:   "foo123bar",
			want: []string{"foo", "bar"},
		},
	}
	for _, tc := range cases {
		got := tokenise(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("tokenise(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("tokenise(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

// ---- Load ----

func TestLoad_ValidVocab(t *testing.T) {
	r := strings.NewReader("hello 1.0 0.0\nworld 0.0 1.0\n")
	emb, err := Load(r)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if emb.Dim() != 2 {
		t.Errorf("want dim=2, got %d", emb.Dim())
	}
	if emb.Len() != 2 {
		t.Errorf("want len=2, got %d", emb.Len())
	}
}

func TestLoad_FastTextHeader(t *testing.T) {
	// FastText files start with "<vocab_size> <dim>" which must be skipped.
	r := strings.NewReader("3 2\nhello 1.0 0.0\nworld 0.0 1.0\n")
	emb, err := Load(r)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if emb.Len() != 2 {
		t.Errorf("want 2 vectors (header skipped), got %d", emb.Len())
	}
}

func TestLoad_EmptyReader(t *testing.T) {
	_, err := Load(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty reader, got nil")
	}
}

func TestLoad_MalformedFloat(t *testing.T) {
	_, err := Load(strings.NewReader("word notafloat\n"))
	if err == nil {
		t.Error("expected error for malformed float, got nil")
	}
}

// ---- Embed ----

func TestEmbed_KnownTokens(t *testing.T) {
	r := strings.NewReader("player 1.0 0.0\ngame 0.0 1.0\n")
	emb, _ := Load(r)

	vec := emb.Embed("player game")
	if vec == nil {
		t.Fatal("expected non-nil embedding for known tokens")
	}
	if len(vec) != 2 {
		t.Errorf("want dim=2, got %d", len(vec))
	}
}

func TestEmbed_UnknownTokens(t *testing.T) {
	r := strings.NewReader("player 1.0 0.0\n")
	emb, _ := Load(r)

	vec := emb.Embed("xyzzy foobar quux")
	if vec != nil {
		t.Errorf("want nil for all-unknown tokens, got %v", vec)
	}
}

func TestEmbed_IsUnitNormalized(t *testing.T) {
	r := strings.NewReader("player 3.0 4.0\n") // magnitude 5, not unit
	emb, _ := Load(r)

	vec := emb.Embed("player")
	if vec == nil {
		t.Fatal("nil embedding")
	}

	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 1e-5 {
		t.Errorf("embedding not unit-normalized: magnitude=%.6f", norm)
	}
}

func TestEmbed_CamelCaseQuery(t *testing.T) {
	// "updatePlayer" should split and match "update" and "player" in vocab.
	r := strings.NewReader("update 1.0 0.0\nplayer 0.0 1.0\n")
	emb, _ := Load(r)

	vec := emb.Embed("updatePlayer")
	if vec == nil {
		t.Error("expected non-nil: camelCase query should split and match vocab tokens")
	}
}
