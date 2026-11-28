package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestReverse(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"cycle and isolated file", `{"a.php":["b.php","c.php","b.php"],"b.php":["a.php"],"c.php":["c.php"],"alone.php":[]}`,
			`{"a.php":["b.php"],"alone.php":[],"b.php":["a.php"],"c.php":["a.php","c.php"]}`},
		{"edge array", `[{"from":"a","to":"b"},{"from":"a","to":"b"},{"from":"c","to":"b"}]`,
			`{"a":[],"b":["a","c"],"c":[]}`},
		{"empty", `{}`, `{}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run([]string{"reverse"}, strings.NewReader(tt.input), &out)
			if err != nil || strings.TrimSpace(out.String()) != tt.want {
				t.Fatalf("output %q, error %v, want %q", out.String(), err, tt.want)
			}
		})
	}
}

func TestReverseDOT(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"reverse", "--format", "dot"}, strings.NewReader(`{"a.php":["b.php"],"b.php":["a.php"],"isolated.php":[]}`), &out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`digraph imports {`, `shape=box`, `label="isolated.php"`, `tooltip="isolated.php"`, "n0 -> n1;", "n1 -> n0;"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, &out)
		}
	}
}

func TestReverseDOTWithoutLabels(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"reverse", "--format", "dot", "--no-labels"}, strings.NewReader(`{"src/a.php":["src/b.php"]}`), &out)
	if err != nil {
		t.Fatal(err)
	}
	dot := out.String()
	if !strings.Contains(dot, "shape=point") || !strings.Contains(dot, `tooltip="src/a.php"`) ||
		strings.Contains(dot, `label="src/a.php"`) || !strings.Contains(dot, "n1 -> n0;") {
		t.Fatalf("unexpected DOT: %s", dot)
	}
}

func TestReverseRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{`null`, `{"a":null}`, `{"a":[""]}`, `{} {}`, `[{"from":"a"}]`} {
		var out bytes.Buffer
		if err := Run([]string{"reverse"}, strings.NewReader(input), &out); err == nil || out.Len() != 0 {
			t.Errorf("input %s produced %q, error %v", input, &out, err)
		}
	}
}
