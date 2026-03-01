package indexer

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	clang "github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Symbol is a structural element extracted from source code.
type Symbol struct {
	Kind      string // function, method, class, type, …
	Name      string
	Signature string
	StartLine uint32
	EndLine   uint32
}

var parsers = map[string]*sitter.Language{
	"go":         golang.GetLanguage(),
	"python":     python.GetLanguage(),
	"javascript": javascript.GetLanguage(),
	"typescript": typescript.GetLanguage(),
	"rust":       rust.GetLanguage(),
	"c":          clang.GetLanguage(),
}

// extractSymbols parses source with the appropriate Tree-sitter grammar
// and returns a flat list of named symbols.
func extractSymbols(src []byte, lang string) ([]Symbol, error) {
	language, ok := parsers[lang]
	if !ok {
		return nil, nil // unsupported lang → skip silently
	}

	parser := sitter.NewParser()
	parser.SetLanguage(language)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	var syms []Symbol
	collectSymbols(root, src, lang, &syms)
	return syms, nil
}

// collectSymbols recurses the AST and extracts named declarations.
func collectSymbols(node *sitter.Node, src []byte, lang string, syms *[]Symbol) {
	kind := node.Type()
	var sym *Symbol

	switch lang {
	case "go":
		sym = extractGoSymbol(node, src, kind)
	case "python":
		sym = extractPythonSymbol(node, src, kind)
	case "javascript":
		sym = extractJSSymbol(node, src, kind)
	case "typescript":
		sym = extractTSSymbol(node, src, kind)
	case "rust":
		sym = extractRustSymbol(node, src, kind)
	case "c":
		sym = extractCSymbol(node, src, kind)
	}

	if sym != nil {
		*syms = append(*syms, *sym)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		collectSymbols(node.Child(i), src, lang, syms)
	}
}

func extractGoSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	switch kind {
	case "function_declaration", "method_declaration":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		symKind := "function"
		if kind == "method_declaration" {
			symKind = "method"
		}
		return &Symbol{
			Kind:      symKind,
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "type_declaration":
		spec := node.NamedChild(0)
		if spec == nil {
			return nil
		}
		name := childByField(spec, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "type",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	}
	return nil
}

func extractPythonSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	switch kind {
	case "function_definition":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "function",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "class_definition":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "class",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	}
	return nil
}

func extractJSSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	switch kind {
	case "function_declaration":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "function",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "class_declaration":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "class",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "method_definition":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "method",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "variable_declarator":
		name := childByField(node, "name")
		val := childByField(node, "value")
		if name == nil || val == nil {
			return nil
		}
		switch val.Type() {
		case "function", "function_expression", "arrow_function":
			return &Symbol{
				Kind:      "function",
				Name:      name.Content(src),
				Signature: firstLine(node.Content(src)),
				StartLine: node.StartPoint().Row + 1,
				EndLine:   node.EndPoint().Row + 1,
			}
		case "class":
			return &Symbol{
				Kind:      "class",
				Name:      name.Content(src),
				Signature: firstLine(node.Content(src)),
				StartLine: node.StartPoint().Row + 1,
				EndLine:   node.EndPoint().Row + 1,
			}
		}
	}
	return nil
}

// extractTSSymbol handles TypeScript: all JS patterns plus interface, type alias, enum.
func extractTSSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	switch kind {
	case "interface_declaration", "enum_declaration":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "type",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "type_alias_declaration":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "type",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	}
	// Fall through to JS patterns for functions, classes, methods.
	return extractJSSymbol(node, src, kind)
}

// extractRustSymbol handles Rust: functions (including impl methods), structs, enums, traits.
func extractRustSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	switch kind {
	case "function_item":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "function",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	case "struct_item", "enum_item", "trait_item":
		name := childByField(node, "name")
		if name == nil {
			return nil
		}
		return &Symbol{
			Kind:      "type",
			Name:      name.Content(src),
			Signature: firstLine(node.Content(src)),
			StartLine: node.StartPoint().Row + 1,
			EndLine:   node.EndPoint().Row + 1,
		}
	}
	return nil
}

// extractCSymbol handles C: function definitions.
// C declarators can be nested: function_definition → (pointer_declarator)* → function_declarator → name.
func extractCSymbol(node *sitter.Node, src []byte, kind string) *Symbol {
	if kind != "function_definition" {
		return nil
	}
	name := cFunctionName(node)
	if name == nil {
		return nil
	}
	return &Symbol{
		Kind:      "function",
		Name:      name.Content(src),
		Signature: firstLine(node.Content(src)),
		StartLine: node.StartPoint().Row + 1,
		EndLine:   node.EndPoint().Row + 1,
	}
}

func cFunctionName(node *sitter.Node) *sitter.Node {
	decl := childByField(node, "declarator")
	for decl != nil {
		switch decl.Type() {
		case "function_declarator":
			return childByField(decl, "declarator")
		case "pointer_declarator":
			decl = childByField(decl, "declarator")
		default:
			return nil
		}
	}
	return nil
}

func childByField(node *sitter.Node, field string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if node.FieldNameForChild(i) == field {
			return c
		}
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		return s[:idx]
	}
	return s
}
