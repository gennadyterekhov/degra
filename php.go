package main

import (
	"path"
	"strings"
)

type phpSymbol struct {
	kind int
	name string
}

const (
	phpClass    = 1
	phpFunction = 2
	phpConstant = 3
)

func phpEdges(files document, add func(string, string)) error {
	symbols := make(map[phpSymbol]string)
	imports := make(map[string][]phpSymbol)
	for _, from := range files.paths() {
		tree, err := readTree(files, from, func(v any) bool { return array(v) != nil })
		if err != nil {
			return err
		}
		for _, item := range array(tree) {
			stmt := object(item)
			if stmt.text("nodeType") == "Stmt_Namespace" {
				namespace := phpName(stmt, "name")
				for _, child := range stmt.list("stmts") {
					phpStatement(from, namespace, object(child), symbols, imports)
				}
			} else {
				phpStatement(from, "", stmt, symbols, imports)
			}
		}
		walk(tree, func(n node) {
			if n.text("nodeType") != "Expr_Include" {
				return
			}
			value, magic, ok := phpInclude(n.child("expr"), "/"+from)
			if !ok || value == "" || strings.ContainsRune(value, 0) {
				return
			}
			if path.IsAbs(value) {
				if magic {
					add(from, path.Clean(strings.TrimLeft(value, "/")))
				}
				return
			}
			if local := path.Join(path.Dir(from), value); files.contains(local) {
				add(from, local)
			} else {
				add(from, path.Clean(value))
			}
		})
	}
	for from, refs := range imports {
		for _, ref := range refs {
			add(from, symbols[ref])
		}
	}
	return nil
}

func phpStatement(from, namespace string, stmt node, symbols map[phpSymbol]string, imports map[string][]phpSymbol) {
	register := func(kind int, name string) {
		if name == "" {
			return
		}
		key := phpKey(kind, phpQualified(namespace, name))
		if _, exists := symbols[key]; !exists {
			symbols[key] = from
		}
	}
	switch stmt.text("nodeType") {
	case "Stmt_Class", "Stmt_Interface", "Stmt_Trait", "Stmt_Enum":
		register(phpClass, phpName(stmt, "name"))
	case "Stmt_Function":
		register(phpFunction, phpName(stmt, "name"))
	case "Stmt_Const":
		for _, constant := range stmt.list("consts") {
			register(phpConstant, phpName(object(constant), "name"))
		}
	case "Stmt_Use", "Stmt_GroupUse":
		prefix := phpName(stmt, "prefix")
		for _, use := range stmt.list("uses") {
			item := object(use)
			kind := item.number("type")
			if kind == 0 {
				kind = stmt.number("type")
			}
			name := phpQualified(prefix, phpName(item, "name"))
			imports[from] = append(imports[from], phpKey(kind, name))
		}
	}
}

func phpName(n node, key string) string {
	return n.child(key).text("name")
}

func phpQualified(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "\\" + name
}

func phpKey(kind int, name string) phpSymbol {
	if kind == phpConstant {
		if i := strings.LastIndexByte(name, '\\'); i >= 0 {
			name = strings.ToLower(name[:i]) + name[i:]
		}
	} else {
		name = strings.ToLower(name)
	}
	return phpSymbol{kind, name}
}

func phpInclude(n node, source string) (value string, magic, ok bool) {
	switch n.text("nodeType") {
	case "Scalar_String":
		return n.text("value"), false, true
	case "Scalar_MagicConst_Dir":
		return strings.TrimSuffix(path.Dir(source), "/"), true, true
	case "Scalar_MagicConst_File":
		return source, true, true
	case "Expr_BinaryOp_Concat":
		left, leftMagic, leftOK := phpInclude(n.child("left"), source)
		right, rightMagic, rightOK := phpInclude(n.child("right"), source)
		return left + right, leftMagic || rightMagic, leftOK && rightOK
	default:
		return "", false, false
	}
}
