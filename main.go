package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
)

type edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type document map[string]json.RawMessage

func (d document) paths() []string {
	return slices.Sorted(maps.Keys(d))
}

func (d document) contains(name string) bool {
	_, ok := d[name]
	return ok
}

func run(args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("degra", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	lang := flags.String("lang", "", "AST language")
	goModule := flags.String("go-module", "", "Go module import path")
	fileName := flags.String("file-name", "", "relative path for a single-file AST")
	graphEdges := flags.Bool("graph-edges", false, "output an array of edges")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !slices.Contains([]string{"go", "js", "php", "py"}, *lang) {
		return errors.New("usage: degra --lang go|js|php|py [--go-module PATH] [--file-name PATH] [--graph-edges] < ast.json")
	}

	files, err := readDocument(input, *fileName)
	if err != nil {
		return err
	}
	resolve := map[string]func(document, func(string, string)) error{
		"go":  func(d document, add func(string, string)) error { return goEdges(d, *goModule, add) },
		"js":  jsEdges,
		"php": phpEdges,
		"py":  pythonEdges,
	}[*lang]

	seen := make(map[edge]struct{})
	if err := resolve(files, func(from, to string) {
		if files.contains(to) {
			seen[edge{From: from, To: to}] = struct{}{}
		}
	}); err != nil {
		return err
	}
	edges := make([]edge, 0, len(seen))
	for e := range seen {
		edges = append(edges, e)
	}
	sortEdges(edges)
	encoder := json.NewEncoder(output)
	if *graphEdges {
		return encoder.Encode(edges)
	}

	imports := make(map[string][]string)
	for _, e := range edges {
		imports[e.From] = append(imports[e.From], e.To)
	}
	return encoder.Encode(imports)
}

func sortEdges(edges []edge) {
	slices.SortFunc(edges, func(a, b edge) int {
		if order := strings.Compare(a.From, b.From); order != 0 {
			return order
		}
		return strings.Compare(a.To, b.To)
	})
}

func readDocument(input io.Reader, fileName string) (document, error) {
	decoder := json.NewDecoder(input)
	var files document
	if fileName == "" {
		if err := decoder.Decode(&files); err != nil || files == nil {
			return nil, fmt.Errorf("expected an AST object keyed by file path: %v", err)
		}
	} else {
		var tree json.RawMessage
		if err := decoder.Decode(&tree); err != nil {
			return nil, err
		}
		files = document{fileName: tree}
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

func main() {
	var err error
	if len(os.Args) == 3 && os.Args[1] == "--project" {
		err = runProject(os.Args[2], os.Stdin, os.Stdout)
	} else {
		err = run(os.Args[1:], os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
