package csdf

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// Op is the tag distinguishing the process expressions of a composition tree.
type Op string

const (
	OpRefer             Op = "REFER"
	OpHide              Op = "HIDE"
	OpInterfaceParallel Op = "INTERFACE_PARALLEL"
)

// Expr is a process expression of a composition tree: a reference to an
// existing diagram, a hiding, or an interface parallel composition. The
// unexported method closes the set of implementations, so a type switch over
// an Expr is exhaustive.
type Expr interface {
	isExpr()
}

// ReferExpr refers to a Composable State Diagram stored at Path.
type ReferExpr struct {
	Path string `json:"path"`
}

func (e *ReferExpr) isExpr() {}

// HideExpr hides Events of Proc (CSP hiding, P \ A). Proc is an interface, so
// the nested expression is read by parseExpr rather than by encoding/json.
type HideExpr struct {
	Events []Event
	Proc   Expr
}

func (e *HideExpr) isExpr() {}

// InterfaceParallelExpr composes Procs in parallel synchronising on Sync (CSP
// interface parallel). Procs holds interfaces, so the nested expressions are
// read by parseExpr rather than by encoding/json.
type InterfaceParallelExpr struct {
	Sync  []Event
	Procs []Expr
}

func (e *InterfaceParallelExpr) isExpr() {}

// ParseExpr parses the JSON representation of a composition tree.
func ParseExpr(bs []byte) (Expr, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(bs, &raw); err != nil {
		return nil, fmt.Errorf("csdf.ParseExpr: %w", err)
	}
	return parseExpr(raw)
}

func parseExpr(raw json.RawMessage) (Expr, error) {
	var tagged struct {
		Op Op `json:"op"`
	}
	if err := json.Unmarshal(raw, &tagged); err != nil {
		return nil, fmt.Errorf("cannot read the op of a process expression: %w", err)
	}

	switch tagged.Op {
	case OpRefer:
		var e ReferExpr
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read a %s expression: %w", OpRefer, err)
		}
		if e.Path == "" {
			return nil, fmt.Errorf("a %s expression requires a non-empty path", OpRefer)
		}
		return &e, nil

	case OpHide:
		var e struct {
			Events []Event         `json:"events"`
			Proc   json.RawMessage `json:"proc"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read a %s expression: %w", OpHide, err)
		}
		proc, err := parseExpr(e.Proc)
		if err != nil {
			return nil, fmt.Errorf("in the proc of a %s expression: %w", OpHide, err)
		}
		return &HideExpr{Events: e.Events, Proc: proc}, nil

	case OpInterfaceParallel:
		var e struct {
			Sync  []Event           `json:"sync"`
			Procs []json.RawMessage `json:"procs"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read an %s expression: %w", OpInterfaceParallel, err)
		}
		if len(e.Procs) < 1 {
			return nil, fmt.Errorf("an %s expression requires at least one proc", OpInterfaceParallel)
		}
		procs := make([]Expr, 0, len(e.Procs))
		for i, rawProc := range e.Procs {
			proc, err := parseExpr(rawProc)
			if err != nil {
				return nil, fmt.Errorf("in procs[%d] of an %s expression: %w", i, OpInterfaceParallel, err)
			}
			procs = append(procs, proc)
		}
		return &InterfaceParallelExpr{Sync: e.Sync, Procs: procs}, nil

	default:
		return nil, fmt.Errorf("unknown op: %q", tagged.Op)
	}
}

// DiagramLoader resolves the path of a REFER expression to a diagram.
type DiagramLoader func(path string) (*Diagram, error)

// BaseDirOf returns the directory the relative paths of REFER expressions are
// resolved against for a composition tree stored at treePath: the directory
// holding the tree. An empty treePath means the tree has no location of its own
// (it was read from standard input), whose base is the current directory.
func BaseDirOf(treePath string) string {
	if treePath == "" {
		return "."
	}
	return filepath.Dir(treePath)
}

// NewFileDiagramLoader loads referenced diagrams from the file system,
// resolving relative paths against baseDir.
func NewFileDiagramLoader(baseDir string) DiagramLoader {
	return func(path string) (*Diagram, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		return LoadDiagram(path)
	}
}

// NewMapDiagramLoader loads referenced diagrams from PlantUML sources held in
// memory, keyed by the path that refers to them. It is the counterpart of
// NewFileDiagramLoader for a caller that has the sources already — a test, or a
// composition assembled in memory.
func NewMapDiagramLoader(sources map[string]string) DiagramLoader {
	return func(path string) (*Diagram, error) {
		source, ok := sources[path]
		if !ok {
			return nil, fmt.Errorf("no such diagram: %q", path)
		}
		return Parse(source)
	}
}

// ComposeTree evaluates a composition tree into a single diagram, loading the
// diagrams of its REFER leaves with load.
func ComposeTree(expr Expr, load DiagramLoader) (*Diagram, error) {
	diagram, err := composeTree(expr, load)
	if err != nil {
		return nil, fmt.Errorf("csdf.ComposeTree: %w", err)
	}
	return diagram, nil
}

func composeTree(expr Expr, load DiagramLoader) (*Diagram, error) {
	switch e := expr.(type) {
	case *ReferExpr:
		diagram, err := load(e.Path)
		if err != nil {
			return nil, fmt.Errorf("in a %s expression of %q: %w", OpRefer, e.Path, err)
		}
		return diagram, nil

	case *HideExpr:
		proc, err := composeTree(e.Proc, load)
		if err != nil {
			return nil, err
		}
		return Hide(proc, e.Events), nil

	case *InterfaceParallelExpr:
		procs := make([]*Diagram, 0, len(e.Procs))
		for _, procExpr := range e.Procs {
			proc, err := composeTree(procExpr, load)
			if err != nil {
				return nil, err
			}
			procs = append(procs, proc)
		}
		composite, err := ComposeParallel(procs, e.Sync)
		if err != nil {
			return nil, fmt.Errorf("in an %s expression: %w", OpInterfaceParallel, err)
		}
		return composite, nil

	default:
		return nil, fmt.Errorf("unknown process expression: %T", expr)
	}
}
