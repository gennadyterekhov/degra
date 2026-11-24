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
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !slices.Contains([]string{"go", "js", "php", "py"}, *lang) {
		return errors.New("usage: degra --lang go|js|php|py [--go-module PATH] [--file-name PATH] < ast.json")
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
	slices.SortFunc(edges, func(a, b edge) int {
		if order := strings.Compare(a.From, b.From); order != 0 {
			return order
		}
		return strings.Compare(a.To, b.To)
	})
	return json.NewEncoder(output).Encode(edges)
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
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
