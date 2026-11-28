package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/gennadyterekhov/degra/internal/graph"
)

func runReverse(args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("degra reverse", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "json", "output format: json or dot")
	noLabels := flags.Bool("no-labels", false, "hide file labels in DOT output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*format != "json" && *format != "dot") {
		return errors.New("usage: degra reverse [--format json|dot] [--no-labels] < imports.json")
	}
	reversed, err := graph.Reverse(input)
	if err != nil {
		return err
	}
	if *format == "dot" {
		return reversed.WriteDOT(output, !*noLabels)
	}
	return json.NewEncoder(output).Encode(reversed)
}
