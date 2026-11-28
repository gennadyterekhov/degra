package graph

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed graph.html
var graphHTML string

var report = template.Must(template.New("graph").Parse(graphHTML))

func (a Adjacency) WriteHTML(output io.Writer) error {
	return report.Execute(output, a)
}
