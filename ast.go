package main

import (
	"encoding/json"
	"fmt"
)

type node map[string]any

func object(value any) node {
	result, _ := value.(map[string]any)
	return result
}

func array(value any) []any {
	result, _ := value.([]any)
	return result
}

func (n node) child(key string) node { return object(n[key]) }
func (n node) list(key string) []any { return array(n[key]) }

func (n node) text(key string) string {
	result, _ := n[key].(string)
	return result
}

func (n node) number(key string) int {
	result, _ := n[key].(float64)
	return int(result)
}

func walk(value any, visit func(node)) {
	switch value := value.(type) {
	case map[string]any:
		visit(value)
		for key, child := range value {
			if key != "attributes" {
				walk(child, visit)
			}
		}
	case []any:
		for _, child := range value {
			walk(child, visit)
		}
	}
}

func readTree(files document, name string, valid func(any) bool) (any, error) {
	var tree any
	if err := json.Unmarshal(files[name], &tree); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if !valid(tree) {
		return nil, fmt.Errorf("%s: unexpected AST root for selected language", name)
	}
	return tree, nil
}
