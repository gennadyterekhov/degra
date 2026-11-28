package graph

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// WriteDOT puts full paths in tooltips and optionally in labels.
func (a Adjacency) WriteDOT(output io.Writer, showLabels bool) error {
	nodes := slices.Sorted(maps.Keys(a))
	var dot strings.Builder
	dot.WriteString("digraph imports {\n")
	if showLabels {
		dot.WriteString("  node [shape=box, style=rounded, fontsize=10];\n")
	} else {
		dot.WriteString("  node [shape=point, width=0.12, label=\"\"];\n")
	}
	dot.WriteString("  edge [color=\"#64748b80\", arrowsize=0.45];\n")
	ids := make(map[string]int, len(nodes))
	groups := make(map[string]string)
	colors := []string{"#2563eb", "#c2410c", "#15803d", "#7e22ce", "#be185d", "#0e7490"}
	for i, node := range nodes {
		ids[node] = i
		group, _, _ := strings.Cut(node, "/")
		color, exists := groups[group]
		if !exists {
			color = colors[len(groups)%len(colors)]
			groups[group] = color
		}
		path := strconv.Quote(node)
		fmt.Fprintf(&dot, "  n%d [color=%q, tooltip=%s", i, color, path)
		if showLabels {
			fmt.Fprintf(&dot, ", label=%s", path)
		}
		dot.WriteString("];\n")
	}
	for i, from := range nodes {
		for _, to := range a[from] {
			fmt.Fprintf(&dot, "  n%d -> n%d;\n", i, ids[to])
		}
	}
	dot.WriteString("}\n")
	_, err := io.WriteString(output, dot.String())
	return err
}
