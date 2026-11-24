package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestGraphs(t *testing.T) {
	tests := []struct {
		lang, module, input, want string
	}{
		{"go", "example.org/project", `{
			"main.go":{"Name":{"Name":"main"},"Imports":[{"Path":{"Value":"\"fmt\""}},{"Path":{"Value":"\"example.org/project/lib\""}},{"Path":{"Value":"\"example.org/project/lib\""}},{"Path":{"Value":"\"other.org/lib\""}}]},
			"lib/a.go":{"Name":{"Name":"lib"}},"lib/b.go":{"Name":{"Name":"lib"}},"lib/a_test.go":{"Name":{"Name":"lib_test"}}
		}`, `[{"from":"main.go","to":"lib/a.go"},{"from":"main.go","to":"lib/b.go"}]`},
		{"js", "", `{
			"src/main.ts":{"kind":308,"statements":[
				{"kind":273,"moduleSpecifier":{"kind":11,"text":"./a.js"}},
				{"kind":279,"moduleSpecifier":{"kind":11,"text":"./a.js"}},
				{"kind":214,"expression":{"kind":80,"escapedText":"require"},"arguments":[{"kind":11,"text":"./b"}]},
				{"kind":214,"expression":{"kind":102},"arguments":[{"kind":15,"text":"./c"}]},
				{"kind":206,"argument":{"kind":202,"literal":{"kind":11,"text":"./d"}}},
				{"kind":284,"expression":{"kind":11,"text":"./d"}},
				{"kind":273,"moduleSpecifier":{"kind":11,"text":"node:fs"}}
			]},
			"src/a.ts":{"kind":308},"src/b/index.js":{"kind":308},"src/c.mjs":{"kind":308},"src/d.ts":{"kind":308}
		}`, `[{"from":"src/main.ts","to":"src/a.ts"},{"from":"src/main.ts","to":"src/b/index.js"},{"from":"src/main.ts","to":"src/c.mjs"},{"from":"src/main.ts","to":"src/d.ts"}]`},
		{"php", "", `{
			"app/main.php":[{"nodeType":"Stmt_Namespace","name":{"name":"App"},"stmts":[
				{"nodeType":"Stmt_GroupUse","type":0,"prefix":{"name":"lib"},"uses":[{"type":1,"name":{"name":"Thing"}},{"type":2,"name":{"name":"make"}}]},
				{"nodeType":"Stmt_Expression","expr":{"nodeType":"Expr_Include","expr":{"nodeType":"Expr_BinaryOp_Concat","left":{"nodeType":"Scalar_MagicConst_Dir"},"right":{"nodeType":"Scalar_String","value":"/helper.php"}}}}
			]}],
			"lib.php":[{"nodeType":"Stmt_Namespace","name":{"name":"Lib"},"stmts":[{"nodeType":"Stmt_Class","name":{"name":"Thing"}},{"nodeType":"Stmt_Function","name":{"name":"make"}}]}],
			"app/helper.php":[]
		}`, `[{"from":"app/main.php","to":"app/helper.php"},{"from":"app/main.php","to":"lib.php"}]`},
		{"py", "", `{
			"src/pkg/main.py":{"_type":"Module","body":[
				{"_type":"Import","names":[{"_type":"alias","name":"importlib","asname":"il"}]},
				{"_type":"ImportFrom","level":1,"module":null,"names":[{"_type":"alias","name":"sub"}]},
				{"_type":"ImportFrom","level":0,"module":"pkg","names":[{"_type":"alias","name":"attribute"}]},
				{"_type":"Call","func":{"_type":"Attribute","attr":"import_module","value":{"_type":"Name","id":"il"}},"args":[{"_type":"Constant","value":"pkg.extra"}],"keywords":[]},
				{"_type":"Import","names":[{"_type":"alias","name":"os"}]}
			]},
			"src/pkg/__init__.py":{"_type":"Module"},"src/pkg/sub.py":{"_type":"Module"},"src/pkg/extra.py":{"_type":"Module"}
		}`, `[{"from":"src/pkg/main.py","to":"src/pkg/__init__.py"},{"from":"src/pkg/main.py","to":"src/pkg/extra.py"},{"from":"src/pkg/main.py","to":"src/pkg/sub.py"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			var out bytes.Buffer
			if err := run([]string{"--graph-edges", "--lang", tt.lang, "--go-module", tt.module}, strings.NewReader(tt.input), &out); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(out.String()); got != tt.want {
				t.Fatalf("got %s; want %s", got, tt.want)
			}
		})
	}
}

func TestGroupedGraph(t *testing.T) {
	input := `{
		"z.py":{"_type":"Module","body":[{"_type":"Import","names":[{"name":"b"},{"name":"a"},{"name":"a"}]}]},
		"a.py":{"_type":"Module","body":[{"_type":"Import","names":[{"name":"b"}]}]},
		"b.py":{"_type":"Module"}
	}`
	var out bytes.Buffer
	if err := run([]string{"--lang", "py"}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	want := "{\"a.py\":[\"b.py\"],\"z.py\":[\"a.py\",\"b.py\"]}\n"
	if out.String() != want {
		t.Fatalf("got %q; want %q", out.String(), want)
	}
}

func TestSingleFile(t *testing.T) {
	for _, tt := range []struct{ flag, want string }{{"", "{}\n"}, {"--graph-edges", "[]\n"}} {
		var out bytes.Buffer
		args := []string{"--lang", "php", "--file-name", "main.php"}
		if tt.flag != "" {
			args = append(args, tt.flag)
		}
		err := run(args, strings.NewReader(`[]`), &out)
		if err != nil || out.String() != tt.want {
			t.Fatalf("output %q, error %v", out.String(), err)
		}
	}
}

func TestInvalidInput(t *testing.T) {
	for _, input := range []string{`[]`, `{"a.py":{"kind":308}}`, `{"a.py":{"_type":"Module"}} {}`} {
		var out bytes.Buffer
		if err := run([]string{"--lang", "py"}, strings.NewReader(input), &out); err == nil || out.Len() != 0 {
			t.Errorf("input %q produced %q, error %v", input, out.String(), err)
		}
	}
}
