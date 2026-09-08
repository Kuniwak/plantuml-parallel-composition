package csdf

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

type ID string
type StateID ID

type Event string

type Var ID

type StateVar struct {
	Name Var    `json:"name"`
	Type string `json:"type,omitempty"`
}

type Predicate string

const (
	PredicateTrue Predicate = "true"
)

// IsTrue reports whether a predicate string is the omitted/default value,
// which renders as the literal True rather than an opaque symbol. The capitalised
// "True"/"False" written by an author are ordinary natural-language predicates.
func IsTrue(s Predicate) bool {
	return s == "" || s == PredicateTrue
}

// Tau is the internal (silent) event. An edge whose event is exactly "tau" is a
// τ-transition (docs/SYNTAX.md, docs/REFINEMENT_ALGORITHM.md §8).
const Tau Event = "tau"

type Diagram struct {
	// Name is the optional PlantUML diagram name written on the @startuml line.
	// It carries no meaning, but it is preserved so that formatting round-trips.
	Name      string            `json:"name,omitempty"`
	States    map[StateID]State `json:"states"`
	StartEdge StartEdge         `json:"start_edge"`
	Edges     []Edge            `json:"edges"`
	EndEdge   *EndEdge          `json:"end_edge"`

	// Promotion directives (docs/PROMOTION.md). They are empty in every
	// diagram that has been through csdfpromote, which is the only tool that
	// accepts them.
	Promotes   []Promote   `json:"promotes"`
	Syncs      []Sync      `json:"syncs"`
	Constrains []Constrain `json:"constrains"`
}

type State struct {
	Name string     `json:"name"`
	Vars []StateVar `json:"vars"`
	Line int        `json:"line"` // 1-based source line of the start edge.
}

func CompareState(a, b State) int {
	return a.Line - b.Line
}

type StateWithID struct {
	State
	ID StateID `json:"id"`
}

// CompareStateWithID orders states by source line, falling back to the id. The
// fallback is what makes SortedStates deterministic: normalize and composition
// synthesise states that never came from a source line, so every such state has
// line 0 and comparing lines alone leaves them all tied.
func CompareStateWithID(a, b StateWithID) int {
	if c := CompareState(a.State, b.State); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func SortedStates(m map[StateID]State) []StateWithID {
	ss := make([]StateWithID, 0, len(m))
	for id, s := range m {
		ss = append(ss, StateWithID{
			ID:    id,
			State: s,
		})
	}
	slices.SortFunc(ss, CompareStateWithID)
	return ss
}

// CompareEdge is the canonical order of transitions: source, event,
// destination, guard, then post. Parallel edges differing only in their
// predicates are common, so the predicates are part of the order; without them
// the order would not be total and the output would not be deterministic.
func CompareEdge(a, b Edge) int {
	if c := cmp.Compare(a.Src, b.Src); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Event, b.Event); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Dst, b.Dst); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Guard, b.Guard); c != 0 {
		return c
	}
	return cmp.Compare(a.Post, b.Post)
}

func SortEdges(edges []Edge) {
	slices.SortFunc(edges, CompareEdge)
}

type StartEdge struct {
	Dst  StateID   `json:"dst"`
	Post Predicate `json:"post"`
	Line int       `json:"line"` // 1-based source line of the start edge.
}

type Edge struct {
	Src   StateID   `json:"src"`
	Dst   StateID   `json:"dst"`
	Event Event     `json:"event"`
	Guard Predicate `json:"guard"`
	Post  Predicate `json:"post"`
	Line  int       `json:"line"` // 1-based source line of the transition.
}

type EndEdge struct {
	Src   StateID   `json:"src"`
	Guard Predicate `json:"guard"`
	Line  int       `json:"line"` // 1-based source line of the transition.
}

// Clone returns a deep copy of the diagram: the result shares no state map,
// state variables, edge slice or end edge with the original, so a caller may
// rewrite the copy without touching its input.
func (d *Diagram) Clone() *Diagram {
	states := make(map[StateID]State, len(d.States))
	for id, state := range d.States {
		state.Vars = append([]StateVar{}, state.Vars...)
		states[id] = state
	}

	var endEdge *EndEdge
	if d.EndEdge != nil {
		copied := *d.EndEdge
		endEdge = &copied
	}

	promotes := make([]Promote, 0, len(d.Promotes))
	for _, promote := range d.Promotes {
		promote.In = append([]StateID{}, promote.In...)
		promotes = append(promotes, promote)
	}

	syncs := make([]Sync, 0, len(d.Syncs))
	for _, sync := range d.Syncs {
		sync.Targets = append([]MapRef{}, sync.Targets...)
		syncs = append(syncs, sync)
	}

	constrains := make([]Constrain, 0, len(d.Constrains))
	for _, constrain := range d.Constrains {
		constrain.Params = append([]string{}, constrain.Params...)
		constrains = append(constrains, constrain)
	}

	return &Diagram{
		Name:       d.Name,
		States:     states,
		StartEdge:  d.StartEdge,
		Edges:      append([]Edge{}, d.Edges...),
		EndEdge:    endEdge,
		Promotes:   promotes,
		Syncs:      syncs,
		Constrains: constrains,
	}
}

func (d *Diagram) String() string {
	return d.StringWithEdgeComments(nil)
}

// StringWithEdgeComments is String with a PlantUML line comment printed before
// an edge, taken from comments at the index of the edge. An empty entry, and an
// index past the end of comments, print no comment. A generated diagram uses it
// to record where each of its edges came from.
func (d *Diagram) StringWithEdgeComments(comments []string) string {
	var sb strings.Builder
	if d.Name == "" {
		sb.WriteString("@startuml\n")
	} else {
		sb.WriteString(fmt.Sprintf("@startuml %s\n", d.Name))
	}

	ss := SortedStates(d.States)

	for _, state := range ss {
		sb.WriteString(fmt.Sprintf("state \"%s\" as %s\n", state.Name, state.ID))
		for _, v := range state.Vars {
			sb.WriteString(fmt.Sprintf("%s: %s", state.ID, v.Name))
			if v.Type != "" {
				sb.WriteString(fmt.Sprintf(" ; %s", v.Type))
			}
			sb.WriteString("\n")
		}
	}

	// StartEdge
	if IsTrue(d.StartEdge.Post) {
		sb.WriteString(fmt.Sprintf("[*] --> %s\n", d.StartEdge.Dst))
	} else {
		sb.WriteString(fmt.Sprintf("[*] --> %s : %s\n", d.StartEdge.Dst, d.StartEdge.Post))
	}

	// Regular edges
	for i, edge := range d.Edges {
		if i < len(comments) && comments[i] != "" {
			sb.WriteString(fmt.Sprintf("' %s\n", comments[i]))
		}
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", edge.Src, edge.Dst, edge.Event))
		// A lone "; x" is a guard (docs/SYNTAX.md), so an edge that only has a
		// post must spell the omitted guard out as "; true ; x".
		if IsTrue(edge.Post) {
			if !IsTrue(edge.Guard) {
				sb.WriteString(fmt.Sprintf(" ; %s", edge.Guard))
			}
			sb.WriteString("\n")
			continue
		}
		guard := edge.Guard
		if IsTrue(guard) {
			guard = PredicateTrue
		}
		sb.WriteString(fmt.Sprintf(" ; %s ; %s\n", guard, edge.Post))
	}

	if d.EndEdge != nil {
		sb.WriteString(fmt.Sprintf("%s --> [*]", d.EndEdge.Src))
		if !IsTrue(d.EndEdge.Guard) {
			sb.WriteString(fmt.Sprintf(" : %s", d.EndEdge.Guard))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
