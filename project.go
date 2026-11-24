package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
)

type projectReference struct {
	edge
	Targets        []string `json:"targets,omitempty"`
	Syntax         []string `json:"syntax,omitempty"`
	ResolvedLayers bool     `json:"resolvedLayers,omitempty"`
	Unlocated      bool     `json:"unlocated,omitempty"`
	Specifier      string   `json:"specifier"`
	Line           int      `json:"line"`
	FromLayer      string   `json:"fromLayer,omitempty"`
	ToLayer        string   `json:"toLayer,omitempty"`
}

type projectViolation struct {
	edge
	Line      int    `json:"line,omitempty"`
	FromLayer string `json:"fromLayer"`
	ToLayer   string `json:"toLayer"`
}

type projectRequest struct {
	Language    string
	Root        string
	Interpreter string
	Layers      []string
	Files       []string
	References  []projectReference
}

func runProject(toolRoot string, input io.Reader, output io.Writer) error {
	var request projectRequest
	if err := json.NewDecoder(input).Decode(&request); err != nil {
		return err
	}
	if !slices.Contains([]string{"go", "js", "php", "py"}, request.Language) {
		return errors.New("unknown project language")
	}
	temp, err := os.MkdirTemp("", "degra-project-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	astCommand, err := projectTool("ast-"+request.Language, toolRoot, temp, request.Interpreter)
	if err != nil {
		return err
	}
	files := make(document)
	if len(request.Files) > 0 {
		manifest, err := json.Marshal(request.Files)
		if err != nil {
			return err
		}
		cmd := exec.Command(astCommand[0], append(slices.Clone(astCommand[1:]), "--files-from-stdin")...)
		cmd.Dir = request.Root
		cmd.Stdin = bytes.NewReader(manifest)
		data, err := commandOutput(cmd)
		if err != nil {
			return err
		}
		files, err = readDocument(bytes.NewReader(data), "")
		if err != nil {
			return err
		}
		for _, file := range request.Files {
			if !files.contains(file) {
				return fmt.Errorf("AST parser omitted %s", file)
			}
		}
	}
	edges, references, err := resolveProjectReferences(request.Language, files, request.References)
	if err != nil {
		return err
	}
	graph, err := json.Marshal(edges)
	if err != nil {
		return err
	}

	configPath := filepath.Join(temp, "layers.json")
	if err := writeProjectJSON(configPath, map[string][]string{"layers": request.Layers}); err != nil {
		return err
	}
	referencesPath := filepath.Join(temp, "references.json")
	if err := writeProjectJSON(referencesPath, references); err != nil {
		return err
	}
	checker, err := projectTool("degra-checker", toolRoot, temp, "")
	if err != nil {
		return err
	}
	cmd := exec.Command(checker[0], append(slices.Clone(checker[1:]), "--config", configPath, "--references", referencesPath)...)
	cmd.Stdin = bytes.NewReader(graph)
	data, err := commandOutput(cmd)
	var status *exec.ExitError
	if err != nil && (!errors.As(err, &status) || status.ExitCode() != 1) {
		return err
	}
	var issues []struct {
		Index     int
		FromLayer string
		ToLayer   string
	}
	if err := json.Unmarshal(data, &issues); err != nil {
		return fmt.Errorf("degra-checker: %w", err)
	}
	violations := make([]string, 0, len(issues))
	details := make([]projectViolation, 0, len(issues))
	for _, issue := range issues {
		if issue.Index < 0 || issue.Index >= len(references) {
			return errors.New("degra-checker returned an invalid reference index")
		}
		ref := references[issue.Index]
		details = append(details, projectViolation{
			edge: ref.edge, Line: ref.Line, FromLayer: issue.FromLayer, ToLayer: issue.ToLayer,
		})
		location := ref.From
		if ref.Line > 0 {
			location += ":" + strconv.Itoa(ref.Line)
		}
		violations = append(violations, fmt.Sprintf("%s -> %s: cannot import package from lower layer (%s -> %s)",
			location, ref.Specifier, issue.FromLayer, issue.ToLayer))
	}
	return json.NewEncoder(output).Encode(struct {
		Edges            []edge             `json:"edges"`
		Violations       []string           `json:"violations"`
		ViolationDetails []projectViolation `json:"violationDetails"`
	}{edges, violations, details})
}

func projectTool(name, root, temp, interpreter string) ([]string, error) {
	if directory := os.Getenv("IMPORT_LAYERS_TOOLS"); directory != "" {
		command, err := exec.LookPath(filepath.Join(directory, name))
		return []string{command}, err
	}
	if command, err := exec.LookPath(name); err == nil {
		return []string{command}, nil
	}
	if root == "" {
		return nil, fmt.Errorf("%s not found on PATH", name)
	}
	scripts := map[string][2]string{
		"ast-js":  {"node", "ast-js.js"},
		"ast-php": {"php", "ast-php.php"},
		"ast-py":  {"python3", "ast-py.py"},
	}
	if script, ok := scripts[name]; ok {
		if interpreter == "" {
			interpreter = script[0]
		}
		return []string{interpreter, filepath.Join(root, name, script[1])}, nil
	}
	binary := filepath.Join(temp, name)
	_, err := commandOutput(exec.Command("go", "-C", filepath.Join(root, name), "build", "-o", binary, "."))
	return []string{binary}, err
}

func commandOutput(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%s: %s%w", filepath.Base(cmd.Path), stderr.String(), err)
	}
	return stdout.Bytes(), nil
}

func writeProjectJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
