package graph

import (
	"path"
	"strings"
)

// SyntaxKind values emitted by TypeScript 5.9.3, as pinned by ast-js.
const (
	tsStringLiteral           = 11
	tsTemplateLiteral         = 15
	tsIdentifier              = 80
	tsImportKeyword           = 102
	tsLiteralType             = 202
	tsImportType              = 206
	tsCallExpression          = 214
	tsImportDeclaration       = 273
	tsExportDeclaration       = 279
	tsExternalModuleReference = 284
	tsSourceFile              = 308
)

func jsEdges(files Document, add func(string, string)) error {
	for _, from := range files.paths() {
		tree, err := readTree(files, from, func(v any) bool { return object(v).number("kind") == tsSourceFile })
		if err != nil {
			return err
		}
		walk(tree, func(n node) {
			spec := jsSpecifier(n)
			if kind := spec.number("kind"); kind == tsStringLiteral || kind == tsTemplateLiteral {
				add(from, jsTarget(files, from, spec.text("text")))
			}
		})
	}
	return nil
}

func jsSpecifier(n node) node {
	switch n.number("kind") {
	case tsImportDeclaration, tsExportDeclaration:
		return n.child("moduleSpecifier")
	case tsExternalModuleReference:
		return n.child("expression")
	case tsImportType:
		if arg := n.child("argument"); arg.number("kind") == tsLiteralType {
			return arg.child("literal")
		}
	case tsCallExpression:
		expr := n.child("expression")
		kind := expr.number("kind")
		args := n.list("arguments")
		if len(args) > 0 && (kind == tsImportKeyword ||
			kind == tsIdentifier && expr.text("escapedText") == "require") {
			return object(args[0])
		}
	}
	return nil
}

var jsExtensions = []string{".js", ".json", ".ts", ".tsx", ".jsx", ".mts", ".cts", ".mjs", ".cjs", ".d.ts"}

func jsTarget(files Document, from, name string) string {
	dir := path.Dir(from)
	switch {
	case name == "", strings.HasPrefix(name, "node:"), strings.Contains(name, ":"):
		return ""
	case name == ".", name == "..", strings.HasPrefix(name, "./"), strings.HasPrefix(name, "../"):
		return jsPath(files, path.Join(dir, name))
	case path.IsAbs(name), strings.HasPrefix(name, "#"):
		return ""
	default:
		for ; ; dir = path.Dir(dir) {
			if to := jsPath(files, path.Join(dir, "node_modules", name)); to != "" {
				return to
			}
			if dir == "." {
				return ""
			}
		}
	}
}

func jsPath(files Document, base string) string {
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	var substitutions []string
	switch ext {
	case ".js":
		substitutions = []string{".ts", ".tsx", ".d.ts"}
	case ".jsx":
		substitutions = []string{".tsx", ".d.ts"}
	case ".mjs":
		substitutions = []string{".mts", ".d.mts"}
	case ".cjs":
		substitutions = []string{".cts", ".d.cts"}
	}
	candidates := []string{base}
	for _, ext := range substitutions {
		candidates = append(candidates, stem+ext)
	}
	for _, ext := range jsExtensions {
		candidates = append(candidates, base+ext)
	}
	for _, ext := range jsExtensions {
		candidates = append(candidates, path.Join(base, "index"+ext))
	}
	for _, candidate := range candidates {
		if files.contains(candidate) {
			return candidate
		}
	}
	return ""
}
