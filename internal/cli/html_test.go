package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReverseHTML(t *testing.T) {
	input := `{"main.php":["lib.php"],"lib.php":["main.php"],"</script><script>alert(1)</script>":[]}`
	var out bytes.Buffer
	if err := Run([]string{"reverse", "--format", "html"}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	const tag = `<script type="application/json" id="graph-data">`
	_, document, found := strings.Cut(out.String(), tag)
	if !found {
		t.Fatal("HTML report has no graph data")
	}
	data, _, found := strings.Cut(document, "</script>")
	if !found {
		t.Fatal("HTML report has no closing graph-data tag")
	}
	var graph map[string][]string
	if err := json.Unmarshal([]byte(data), &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph) != 3 || len(graph["lib.php"]) != 1 || graph["lib.php"][0] != "main.php" {
		t.Fatalf("unexpected graph: %#v", graph)
	}
	if strings.Contains(out.String(), "<script>alert(1)</script>") {
		t.Fatal("file names must be escaped when embedded in HTML")
	}
}
