package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"
)

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Document map[string]json.RawMessage

func (d Document) paths() []string {
	return slices.Sorted(maps.Keys(d))
}

func (d Document) contains(name string) bool {
	_, ok := d[name]
	return ok
}

func Build(files Document, lang, goModule string) ([]Edge, error) {
	resolve := map[string]func(Document, func(string, string)) error{
		"go":  func(d Document, add func(string, string)) error { return goEdges(d, goModule, add) },
		"js":  jsEdges,
		"php": phpEdges,
		"py":  pythonEdges,
	}[lang]

	seen := make(map[Edge]struct{})
	if err := resolve(files, func(from, to string) {
		if files.contains(to) {
			seen[Edge{From: from, To: to}] = struct{}{}
		}
	}); err != nil {
		return nil, err
	}
	edges := make([]Edge, 0, len(seen))
	for e := range seen {
		edges = append(edges, e)
	}
	sortEdges(edges)
	return edges, nil
}

func sortEdges(edges []Edge) {
	slices.SortFunc(edges, func(a, b Edge) int {
		if order := strings.Compare(a.From, b.From); order != 0 {
			return order
		}
		return strings.Compare(a.To, b.To)
	})
}

func ReadDocument(input io.Reader, fileName string) (Document, error) {
	decoder := json.NewDecoder(input)
	var files Document
	if fileName == "" {
		if err := decoder.Decode(&files); err != nil || files == nil {
			return nil, fmt.Errorf("expected an AST object keyed by file path: %v", err)
		}
	} else {
		var tree json.RawMessage
		if err := decoder.Decode(&tree); err != nil {
			return nil, err
		}
		files = Document{fileName: tree}
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("expected exactly one JSON document")
	}
	for name := range files {
		if name == "" || name == "." || name == ".." || path.IsAbs(name) ||
			name != path.Clean(name) || strings.HasPrefix(name, "../") || strings.ContainsRune(name, 0) {
			return nil, fmt.Errorf("invalid relative file path %q", name)
		}
	}
	return files, nil
}
