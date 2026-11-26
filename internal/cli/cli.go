package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"slices"

	"github.com/gennadyterekhov/degra/internal/graph"
	"github.com/gennadyterekhov/degra/internal/pipeline"
)

func Run(args []string, input io.Reader, output io.Writer) error {
	if len(args) == 2 {
		switch args[0] {
		case "--project":
			return pipeline.RunProject(args[1], input, output)
		case "--parse-project":
			return pipeline.ParseProject(args[1], input, output)
		}
	}
	return run(args, input, output)
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

	files, err := graph.ReadDocument(input, *fileName)
	if err != nil {
		return err
	}
	edges, err := graph.Build(files, *lang, *goModule)
	if err != nil {
		return err
	}
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
