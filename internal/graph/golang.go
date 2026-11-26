package graph

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
)

type goAST struct {
	Name    *struct{ Name string }
	Imports []struct {
		Path struct{ Value string }
	}
}

func goEdges(files Document, module string, add func(string, string)) error {
	packages := make(map[string][]string)
	trees := make(map[string]goAST)
	for _, name := range files.paths() {
		var tree goAST
		if err := json.Unmarshal(files[name], &tree); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if tree.Name == nil || tree.Name.Name == "" {
			return fmt.Errorf("%s: expected Go file AST", name)
		}
		trees[name] = tree
		if !strings.HasSuffix(name, "_test.go") {
			dir := path.Dir(name)
			packages[dir] = append(packages[dir], name)
		}
	}
	module = strings.TrimSuffix(module, "/")
	for from, tree := range trees {
		for _, spec := range tree.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: invalid Go import literal: %w", from, err)
			}
			for _, to := range goPackage(packages, module, from, name) {
				add(from, to)
			}
		}
	}
	return nil
}

func goPackage(packages map[string][]string, module, from, name string) []string {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || name == "." || name == ".." {
		return packages[path.Join(path.Dir(from), name)]
	}
	if module != "" {
		if name == module {
			return packages["."]
		}
		if relative, ok := strings.CutPrefix(name, module+"/"); ok {
			return packages[relative]
		}
	}
	// Search vendor directories from the importing package up to the AST root.
	for dir := path.Dir(from); ; dir = path.Dir(dir) {
		if targets := packages[path.Join(dir, "vendor", name)]; len(targets) > 0 {
			return targets
		}
		if dir == "." {
			return nil
		}
	}
}
