package indexer

import (
	"testing"
)

func TestExtractJSSymbols(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantKind string
		wantName string
	}{
		{
			name:     "function_declaration",
			src:      `function foo() {}`,
			wantKind: "function", wantName: "foo",
		},
		{
			name:     "class_declaration",
			src:      `class Foo {}`,
			wantKind: "class", wantName: "Foo",
		},
		{
			name:     "exported class",
			src:      `export class Game {}`,
			wantKind: "class", wantName: "Game",
		},
		{
			name:     "exported function",
			src:      `export function update() {}`,
			wantKind: "function", wantName: "update",
		},
		{
			name:     "method_definition",
			src:      `class Foo { update() {} }`,
			wantKind: "method", wantName: "update",
		},
		{
			name:     "arrow function variable",
			src:      `const shoot = () => {};`,
			wantKind: "function", wantName: "shoot",
		},
		{
			name:     "function expression variable",
			src:      `const init = function() {};`,
			wantKind: "function", wantName: "init",
		},
		{
			name:     "class expression variable",
			src:      `const Player = class {};`,
			wantKind: "class", wantName: "Player",
		},
		{
			// export default wraps function_declaration — recursive walk finds it.
			name:     "export default function",
			src:      `export default function render() {}`,
			wantKind: "function", wantName: "render",
		},
		{
			// export default wraps class_declaration — recursive walk finds it.
			name:     "export default class",
			src:      `export default class Engine {}`,
			wantKind: "class", wantName: "Engine",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			syms, err := extractSymbols([]byte(tc.src), "javascript")
			if err != nil {
				t.Fatalf("extractSymbols: %v", err)
			}
			for _, s := range syms {
				if s.Kind == tc.wantKind && s.Name == tc.wantName {
					return
				}
			}
			t.Errorf("want symbol {kind=%q, name=%q}, got %+v", tc.wantKind, tc.wantName, syms)
		})
	}
}

func TestExtractGoSymbols(t *testing.T) {
	src := `package main

func Hello() string { return "hi" }

type Foo struct{}

func (f *Foo) Bar() {}
`
	syms, err := extractSymbols([]byte(src), "go")
	if err != nil {
		t.Fatalf("extractSymbols: %v", err)
	}

	byName := make(map[string]string)
	for _, s := range syms {
		byName[s.Name] = s.Kind
	}

	want := map[string]string{
		"Hello": "function",
		"Foo":   "type",
		"Bar":   "method",
	}
	for name, kind := range want {
		if byName[name] != kind {
			t.Errorf("want %s=%s, got %q", name, kind, byName[name])
		}
	}
}

func TestExtractPythonSymbols(t *testing.T) {
	src := `
def greet(name):
    pass

class Animal:
    def speak(self):
        pass
`
	syms, err := extractSymbols([]byte(src), "python")
	if err != nil {
		t.Fatalf("extractSymbols: %v", err)
	}

	byName := make(map[string]string)
	for _, s := range syms {
		byName[s.Name] = s.Kind
	}

	want := map[string]string{
		"greet":  "function",
		"Animal": "class",
		"speak":  "function",
	}
	for name, kind := range want {
		if byName[name] != kind {
			t.Errorf("want %s=%s, got %q", name, kind, byName[name])
		}
	}
}

func TestExtractJSClassMethods(t *testing.T) {
	src := `
class Game {
  constructor() {}
  update() {}
  draw() {}
}
`
	syms, err := extractSymbols([]byte(src), "javascript")
	if err != nil {
		t.Fatalf("extractSymbols: %v", err)
	}

	byName := make(map[string]string)
	for _, s := range syms {
		byName[s.Name] = s.Kind
	}

	want := map[string]string{
		"Game":        "class",
		"constructor": "method",
		"update":      "method",
		"draw":        "method",
	}
	for name, kind := range want {
		if byName[name] != kind {
			t.Errorf("want %s=%s, got %q (all syms: %+v)", name, kind, byName[name], syms)
		}
	}
}

func TestExtractUnsupportedLang(t *testing.T) {
	syms, err := extractSymbols([]byte(`fn main() {}`), "rust")
	if err != nil {
		t.Fatal(err)
	}
	if syms != nil {
		t.Errorf("want nil for unsupported lang, got %v", syms)
	}
}
