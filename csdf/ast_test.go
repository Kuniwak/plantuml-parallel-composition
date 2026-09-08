package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDiagramStringOrdersStatesByLine(t *testing.T) {
	// Setup: a map literal whose iteration order is not stable across runs.
	diagram := Diagram{
		States: map[StateID]State{
			"s2": {Name: "Third", Line: 3},
			"s0": {Name: "First", Line: 1},
			"s1": {Name: "Second", Line: 2},
		},
		StartEdge: StartEdge{Dst: "s0", Post: PredicateTrue},
	}
	want := `@startuml
state "First" as s0
state "Second" as s1
state "Third" as s2
[*] --> s0
@enduml
`

	// Execute
	got := diagram.String()

	// Assert
	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}
}

func TestCompareStateWithIDBreaksTiesByID(t *testing.T) {
	// Derived states — those normalize and composition synthesise rather than
	// parse — carry no source line, so line order alone leaves them tied and the
	// sort is free to return them in any order. The id has to settle it.
	a := StateWithID{ID: "s0", State: State{Name: "{s0}"}}
	b := StateWithID{ID: "s1_s2", State: State{Name: "{s1, s2}"}}

	if got := CompareStateWithID(a, b); got >= 0 {
		t.Errorf("CompareStateWithID(s0, s1_s2) = %d, want negative", got)
	}
	if got := CompareStateWithID(b, a); got <= 0 {
		t.Errorf("CompareStateWithID(s1_s2, s0) = %d, want positive", got)
	}
}

func TestDiagramStringIncludesEndEdge(t *testing.T) {
	// Setup
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {Name: "SKIP"},
		},
		StartEdge: StartEdge{Dst: "s0", Post: PredicateTrue},
		EndEdge:   &EndEdge{Src: "s0", Guard: PredicateTrue},
	}
	want := `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*]
@enduml
`

	// Execute
	got := diagram.String()

	// Assert
	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}

	// Teardown: no resources to release.
}

func TestDiagramStringIncludesStateVarTypes(t *testing.T) {
	// Setup
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {
				Name: "Initial",
				Vars: []StateVar{
					{Name: "ready", Type: "bool"},
					{Name: "count"},
				},
			},
		},
		StartEdge: StartEdge{Dst: "s0", Post: PredicateTrue},
	}
	want := `@startuml
state "Initial" as s0
s0: ready ; bool
s0: count
[*] --> s0
@enduml
`

	// Execute
	got := diagram.String()

	// Assert
	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}

	// Teardown: no resources to release.
}

func TestDiagramStringIncludesFreeFormEvent(t *testing.T) {
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {Name: "Initial"},
		},
		StartEdge: StartEdge{Dst: "s0", Post: PredicateTrue},
		Edges: []Edge{
			{Src: "s0", Dst: "s0", Event: "finish(result, status)", Guard: PredicateTrue, Post: PredicateTrue},
		},
	}
	want := `@startuml
state "Initial" as s0
[*] --> s0
s0 --> s0 : finish(result, status)
@enduml
`

	got := diagram.String()

	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}
}

func TestDiagramStringRoundTripsGuardsAndPosts(t *testing.T) {
	// Setup: every combination of an omitted guard and an omitted post must
	// survive a print/parse round trip. `; g` is a guard, `; true ; p` a post.
	source := `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
s0 --> s1 : b ; g
s0 --> s1 : c ; true ; p
s0 --> s1 : d ; g ; p
@enduml
`

	// Execute
	printed := MustParse(source).String()

	// Assert
	if diff := cmp.Diff(source, printed); diff != "" {
		t.Error(diff)
	}
}

func TestStringWithEdgeComments(t *testing.T) {
	// Arrange: a generated diagram says where each of its edges came from, so
	// that a reader can find the source the predicate was written in.
	d := MustParse(`@startuml
state "A" as a
state "B" as b
[*] --> a
a --> b : go
b --> a : back
@enduml
`)

	// Act
	got := d.StringWithEdgeComments([]string{"promote: LOCAL.puml 〈A〉 → 〈B〉", ""})

	// Assert
	want := `@startuml
state "A" as a
state "B" as b
[*] --> a
' promote: LOCAL.puml 〈A〉 → 〈B〉
a --> b : go
b --> a : back
@enduml
`
	if got != want {
		t.Errorf("StringWithEdgeComments() =\n%s\nwant:\n%s", got, want)
	}

	if d.String() != d.StringWithEdgeComments(nil) {
		t.Errorf("String() and StringWithEdgeComments(nil) disagree")
	}
}
