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
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*format != "json" && *format != "dot") {
		return errors.New("usage: degra reverse [--format json|dot] < imports.json")
	}
	reversed, err := graph.Reverse(input)
	if err != nil {
		return err
	}
	if *format == "dot" {
		return reversed.WriteDOT(output)
	}
	return json.NewEncoder(output).Encode(reversed)
}
