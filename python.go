package main

import (
	"path"
	"strings"
	"unicode"
)

func pythonEdges(files document, add func(string, string)) error {
	dirs := make(map[string]bool)
	for name := range files {
		for dir := path.Dir(name); !dirs[dir]; dir = path.Dir(dir) {
			dirs[dir] = true
			if dir == "." {
				break
			}
		}
	}
	for _, from := range files.paths() {
		tree, err := readTree(files, from, func(v any) bool { return object(v).text("_type") == "Module" })
		if err != nil {
			return err
		}

		resolve := func(name string, roots ...string) string {
			return pythonModule(files, dirs, name, roots)
		}
		absoluteRoots := []string{".", "src"}
		dynamic := pythonDynamicImports(tree)
		walk(tree, func(n node) {
			switch n.text("_type") {
			case "Import":
				for _, alias := range n.list("names") {
					add(from, resolve(object(alias).text("name"), absoluteRoots...))
				}
			case "ImportFrom":
				roots := absoluteRoots
				if level := n.number("level"); level != 0 {
					dir := path.Dir(from)
					root := "."
					if strings.HasPrefix(from, "src/") {
						root = "src"
					}
					for i := 1; i < level && dir != root; i++ {
						dir = path.Dir(dir)
					}
					if dir == root || level < 0 {
						return
					}
					roots = []string{dir}
				}
				base := n.text("module")
				for _, alias := range n.list("names") {
					name := object(alias).text("name")
					if base != "" {
						name = base + "." + name
					}
					to := resolve(name, roots...)
					if to == "" {
						to = resolve(base, roots...)
					}
					add(from, to)
				}
			case "Call":
				if name := dynamic(n); name != "" {
					add(from, resolve(name, absoluteRoots...))
				}
			}
		})
	}
	return nil
}

func pythonModule(files document, dirs map[string]bool, name string, roots []string) string {
	if name == "" {
		if len(roots) == 1 {
			return path.Join(roots[0], "__init__.py")
		}
		return ""
	}
	parts := strings.Split(name, ".")
	for i, part := range parts {
		if !pythonIdentifier(part) {
			return ""
		}
		var namespaces []string
		for _, root := range roots {
			pkg := path.Join(root, part)
			source := pkg + ".py"
			if files.contains(path.Join(pkg, "__init__.py")) {
				if i == len(parts)-1 {
					return path.Join(pkg, "__init__.py")
				}
				namespaces = []string{pkg}
				break
			}
			if files.contains(source) {
				if i == len(parts)-1 {
					return source
				}
				return ""
			}
			if dirs[pkg] {
				namespaces = append(namespaces, pkg)
			}
		}
		if len(namespaces) == 0 {
			return ""
		}
		roots = namespaces
	}
	return ""
}

func pythonIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Other_ID_Start) {
			continue
		}
		if i == 0 || !unicode.In(r, unicode.Nd, unicode.Mn, unicode.Mc, unicode.Pc, unicode.Other_ID_Continue) {
			return false
		}
	}
	return true
}

func pythonBinding(alias node) string {
	if name := alias.text("asname"); name != "" {
		return name
	}
	name, _, _ := strings.Cut(alias.text("name"), ".")
	return name
}

func pythonDynamicImports(tree any) func(node) string {
	modules := make(map[string]bool)
	functions := map[string]bool{"__import__": true}
	bindings := make(map[string]int)

	for _, item := range object(tree).list("body") {
		stmt := object(item)
		for _, value := range stmt.list("names") {
			alias := object(value)
			name := pythonBinding(alias)
			switch {
			case stmt.text("_type") == "Import" && alias.text("name") == "importlib":
				modules[name] = true
			case stmt.text("_type") == "ImportFrom" && stmt.number("level") == 0 &&
				stmt.text("module") == "importlib" && alias.text("name") == "import_module":
				functions[name] = true
			default:
				continue
			}
			bindings[name]--
		}
	}
	walk(tree, func(n node) {
		switch n.text("_type") {
		case "alias":
			bindings[pythonBinding(n)]++
		case "Name":
			if pythonAssignment(n) {
				bindings[n.text("id")]++
			}
		case "Attribute":
			if n.text("attr") == "import_module" && pythonAssignment(n) && n.child("value").text("_type") == "Name" {
				bindings[n.child("value").text("id")]++
			}
		case "arg":
			bindings[n.text("arg")]++
		case "FunctionDef", "AsyncFunctionDef", "ClassDef", "ExceptHandler", "MatchAs", "MatchStar":
			bindings[n.text("name")]++
		case "MatchMapping":
			bindings[n.text("rest")]++
		}
	})
	return func(call node) string {
		args := call.list("args")
		if len(args) != 1 || len(call.list("keywords")) != 0 || bindings["*"] > 0 {
			return ""
		}
		arg := object(args[0])
		if arg.text("_type") != "Constant" {
			return ""
		}

		fn := call.child("func")
		switch fn.text("_type") {
		case "Name":
			name := fn.text("id")
			if functions[name] && !modules[name] && bindings[name] == 0 {
				return arg.text("value")
			}
		case "Attribute":
			value := fn.child("value")
			name := value.text("id")
			if fn.text("attr") == "import_module" && value.text("_type") == "Name" &&
				modules[name] && !functions[name] && bindings[name] == 0 {
				return arg.text("value")
			}
		}
		return ""
	}
}

func pythonAssignment(n node) bool {
	context := n.child("ctx").text("_type")
	return context == "Store" || context == "Del"
}
