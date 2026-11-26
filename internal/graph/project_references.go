package graph

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Reference struct {
	Edge
	Targets        []string `json:"targets,omitempty"`
	Syntax         []string `json:"syntax,omitempty"`
	ResolvedLayers bool     `json:"resolvedLayers,omitempty"`
	Unlocated      bool     `json:"unlocated,omitempty"`
	Specifier      string   `json:"specifier"`
	Line           int      `json:"line"`
	FromLayer      string   `json:"fromLayer,omitempty"`
	ToLayer        string   `json:"toLayer,omitempty"`
}

type importSites struct {
	names map[string]bool
	lines map[int]bool
}

// Build the scoped graph by joining AST import sites with the targets that the
// frontend resolved under the project's package, workspace and selection rules.
func ResolveProjectReferences(language string, files Document, candidates []Reference) ([]Edge, []Reference, error) {
	sites := make(map[string]importSites, len(files))
	for name, raw := range files {
		imports, err := findImportSites(language, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		sites[name] = imports
	}

	seen := make(map[Edge]bool)
	edges := []Edge{}
	references := make([]Reference, 0, len(candidates))
	for _, ref := range candidates {
		imports, parsed := sites[ref.From]
		if parsed && !ref.Unlocated && !imports.matches(ref) {
			continue
		}
		targets := ref.Targets
		if ref.To != "" {
			targets = append(targets, ref.To)
		}
		for _, to := range targets {
			e := Edge{ref.From, to}
			if !seen[e] {
				seen[e] = true
				edges = append(edges, e)
			}
		}
		references = append(references, ref)
	}
	sortEdges(edges)
	return edges, references, nil
}

func (sites importSites) matches(ref Reference) bool {
	if len(ref.Syntax) == 0 {
		return sites.lines[ref.Line]
	}
	for _, name := range ref.Syntax {
		if sites.names[name] {
			return true
		}
	}
	return false
}

func findImportSites(language string, raw json.RawMessage) (importSites, error) {
	sites := importSites{names: make(map[string]bool), lines: make(map[int]bool)}
	if language == "go" {
		var tree goAST
		if err := json.Unmarshal(raw, &tree); err != nil {
			return sites, err
		}
		if tree.Name == nil || tree.Name.Name == "" {
			return sites, fmt.Errorf("expected Go file AST")
		}
		for _, spec := range tree.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return sites, err
			}
			sites.names[name] = true
		}
		return sites, nil
	}

	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return sites, err
	}
	switch language {
	case "js":
		if object(tree).number("kind") != tsSourceFile {
			return sites, fmt.Errorf("expected TypeScript source file AST")
		}
		walk(object(tree).list("statements"), func(n node) {
			spec := jsSpecifier(n)
			if kind := spec.number("kind"); kind == tsStringLiteral || kind == tsTemplateLiteral {
				sites.names[spec.text("text")] = true
			}
		})
	case "php":
		if array(tree) == nil {
			return sites, fmt.Errorf("expected PHP statement list AST")
		}
		walk(tree, func(n node) {
			switch n.text("nodeType") {
			case "UseItem", "Stmt_UseUse", "Expr_Include":
				sites.lines[n.child("attributes").number("startLine")] = true
			}
		})
	case "py":
		if object(tree).text("_type") != "Module" {
			return sites, fmt.Errorf("expected Python module AST")
		}
		dynamic := pythonDynamicImports(tree)
		walk(tree, func(n node) {
			kind := n.text("_type")
			if kind == "Import" || kind == "ImportFrom" || kind == "Call" && dynamic(n) != "" {
				sites.lines[n.number("lineno")] = true
			}
		})
	}
	return sites, nil
}
