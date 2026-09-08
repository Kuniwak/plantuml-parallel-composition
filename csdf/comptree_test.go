package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseExprReadsProcessExpressions(t *testing.T) {
	testCases := map[string]struct {
		Source   string
		Expected Expr
	}{
		"refer (the leaf of a composition tree)": {
			Source:   `{"op": "REFER", "path": "path/to/A.puml"}`,
			Expected: &ReferExpr{Path: "path/to/A.puml"},
		},
		"hide (wrapping a nested expression)": {
			Source:   `{"op": "HIDE", "events": ["EVT-A"], "proc": {"op": "REFER", "path": "A.puml"}}`,
			Expected: &HideExpr{Events: []Event{"EVT-A"}, Proc: &ReferExpr{Path: "A.puml"}},
		},
		"interface parallel (holding a list of nested expressions)": {
			Source: `{"op": "INTERFACE_PARALLEL", "sync": ["EVT-A"], "procs": [{"op": "REFER", "path": "A.puml"}, {"op": "REFER", "path": "B.puml"}]}`,
			Expected: &InterfaceParallelExpr{
				Sync:  []Event{"EVT-A"},
				Procs: []Expr{&ReferExpr{Path: "A.puml"}, &ReferExpr{Path: "B.puml"}},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseExpr([]byte(testCase.Source))
			if err != nil {
				t.Fatalf("want nil, got %#v", err)
			}

			if diff := cmp.Diff(testCase.Expected, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestParseExprRejectsInvalidTrees(t *testing.T) {
	testCases := map[string]string{
		"unknown op":                       `{"op": "SEQ", "procs": []}`,
		"refer without a path":             `{"op": "REFER"}`,
		"hide without a proc":              `{"op": "HIDE", "events": ["a"]}`,
		"interface parallel without procs": `{"op": "INTERFACE_PARALLEL", "sync": []}`,
		"broken nested expression":         `{"op": "HIDE", "events": ["a"], "proc": {"op": "REFER"}}`,
		"not an object":                    `["op"]`,
	}

	for name, source := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseExpr([]byte(source))
			if err == nil {
				t.Errorf("want not nil, got nil: %#v", got)
			}
		})
	}
}

func TestComposeTreeComposesAndHidesReferencedDiagrams(t *testing.T) {
	// Setup: the tree of the two example diagrams synchronised on "sync",
	// with "sync" hidden afterwards.
	expr, err := ParseExpr([]byte(`{
		"op": "HIDE",
		"proc": {
			"op": "INTERFACE_PARALLEL",
			"sync": ["sync"],
			"procs": [
				{"op": "REFER", "path": "in.puml"},
				{"op": "REFER", "path": "out.puml"}
			]
		},
		"events": ["sync"]
	}`))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	got, err := ComposeTree(expr, NewFileDiagramLoader("../examples/valid"))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	want := `@startuml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : tau
s2_s1 --> s2_s2 : out
@enduml
`
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Error(diff)
	}
}

func TestComposeTreeReportsUnreadableReferences(t *testing.T) {
	// Setup: a REFER pointing at a missing file must name the path it failed on.
	expr := Expr(&ReferExpr{Path: "missing.puml"})

	_, err := ComposeTree(expr, NewFileDiagramLoader("../examples/valid"))

	if err == nil {
		t.Fatal("want not nil, got nil")
	}
	if !strings.Contains(err.Error(), "missing.puml") {
		t.Errorf("want an error naming missing.puml, got %q", err)
	}
}

func TestBaseDirOf(t *testing.T) {
	testCases := map[string]struct {
		TreePath string
		Expected string
	}{
		"a tree file (representative value)":   {TreePath: "path/to/tree.json", Expected: "path/to"},
		"a tree file in the current directory": {TreePath: "tree.json", Expected: "."},
		"no location, e.g. standard input":     {TreePath: "", Expected: "."},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := BaseDirOf(testCase.TreePath); got != testCase.Expected {
				t.Errorf("want %q, got %q", testCase.Expected, got)
			}
		})
	}
}

func TestComposeTreeFoldsMoreThanTwoProcs(t *testing.T) {
	// Setup: three processes synchronised on the single event they share, as
	// documented in docs/COMPOSITION_TREE.md.
	load := func(path string) (*Diagram, error) {
		return MustParse(`@startuml
state "` + path + `" as s0
state "` + path + `" as s1
[*] --> s0
s0 --> s1 : sync
@enduml`), nil
	}
	expr := Expr(&InterfaceParallelExpr{
		Sync: []Event{"sync"},
		Procs: []Expr{
			&ReferExpr{Path: "A"},
			&ReferExpr{Path: "B"},
			&ReferExpr{Path: "C"},
		},
	})

	got, err := ComposeTree(expr, load)
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	// The three processes can only move together, so the composite has exactly
	// one transition between its two states.
	if want := 1; len(got.Edges) != want {
		t.Errorf("want %d edges, got %d: %s", want, len(got.Edges), got.String())
	}
	if want := 2; len(got.States) != want {
		t.Errorf("want %d states, got %d: %s", want, len(got.States), got.String())
	}
}

func TestNewMapDiagramLoader(t *testing.T) {
	// Arrange
	load := NewMapDiagramLoader(map[string]string{"a.puml": `@startuml
state "A" as a
[*] --> a
a --> a : e
@enduml
`})

	// Act
	diagram, err := load("a.puml")

	// Assert
	if err != nil {
		t.Fatalf("load(\"a.puml\") = _, %v; want no error", err)
	}
	if len(diagram.Edges) != 1 {
		t.Errorf("len(Edges) = %d; want 1", len(diagram.Edges))
	}

	if _, err := load("missing.puml"); err == nil {
		t.Errorf("load(\"missing.puml\") = _, nil; want an error")
	}
}
