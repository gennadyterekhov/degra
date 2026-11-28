package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

type Adjacency map[string][]string

// Reverse inverts direct edges while retaining every node in the input.
func Reverse(input io.Reader) (Adjacency, error) {
	decoder := json.NewDecoder(input)
	var data json.RawMessage
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, errors.New("expected exactly one graph JSON document")
	}

	result := make(Adjacency)
	addNode := func(name string) {
		if _, exists := result[name]; !exists {
			result[name] = []string{}
		}
	}
	addEdge := func(from, to string) error {
		if from == "" || to == "" {
			return errors.New("expected non-empty paths for both edge endpoints")
		}
		addNode(from)
		result[to] = append(result[to], from)
		return nil
	}
	switch data[0] {
	case '{':
		var imports Adjacency
		if err := json.Unmarshal(data, &imports); err != nil {
			return nil, err
		}
		for from, targets := range imports {
			if from == "" || targets == nil {
				return nil, fmt.Errorf("expected a non-empty path and an array of imports for %q", from)
			}
			addNode(from)
			for _, to := range targets {
				if err := addEdge(from, to); err != nil {
					return nil, err
				}
			}
		}
	case '[':
		var edges []Edge
		if err := json.Unmarshal(data, &edges); err != nil {
			return nil, err
		}
		for _, e := range edges {
			if err := addEdge(e.From, e.To); err != nil {
				return nil, err
			}
		}
	default:
		return nil, errors.New("expected an import map or an array of edges")
	}
	for name, importers := range result {
		slices.Sort(importers)
		result[name] = slices.Compact(importers)
	}
	return result, nil
}
