package promote

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// LoadFunc resolves the path written in an !include into the local diagram it
// names. The caller decides what the path is relative to.
type LoadFunc func(path string) (*csdf.Diagram, error)

// Severity is how much a diagnostic matters. An error leaves the diagram
// unprinted, so an unsound expansion never reaches the tools downstream.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// Diagnostic is one thing the expansion has to say about the diagram it read.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Line     int      `json:"line"` // 1-based source line, 0 when there is none.
	Message  string   `json:"message"`
}

func (d Diagnostic) String() string {
	if d.Line == 0 {
		return fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: line %d: %s", d.Severity, d.Line, d.Message)
}

// Expansion is the expanded diagram together with the origin of every edge the
// expansion generated.
//
// It prints itself rather than leaving that to csdf.Diagram, because the origin
// comments have to sit between the edges and the core printer has nowhere to put
// them. The core grammar and its printer are deliberately left alone.
type Expansion struct {
	Diagram *csdf.Diagram `json:"diagram"`
	// Origins is aligned with Diagram.Edges: the comment to print above each
	// edge, empty for an edge the author wrote by hand.
	Origins []string `json:"origins"`
}

// eventRe splits a local event into its name and its argument list. An event is
// a free string, so this is a convention rather than grammar: what is not of
// this shape is promoted by appending the instance ID as the only argument.
var eventRe = regexp.MustCompile(`^([^()]+?)\s*\(\s*(.*?)\s*\)$`)

// Expand turns every promotion into edges on the state its block was written in.
func Expand(g *GlobalDiagram, load LoadFunc) (*Expansion, []Diagnostic, error) {
	e := &expander{global: g, load: load, locals: map[csdf.Var]*csdf.Diagram{}, paths: map[csdf.Var]string{}}
	return e.run()
}

type expander struct {
	global *GlobalDiagram
	load   LoadFunc

	locals map[csdf.Var]*csdf.Diagram
	// paths is where each map's local diagram was read from. A block with no
	// body has no path of its own, but its edges came from the same file.
	paths map[csdf.Var]string
	// order is the block that carries each map's !include, in source order, so
	// the checks and the expansion read the maps in the order they were written.
	order []Promote
	diags []Diagnostic
}

func (e *expander) run() (*Expansion, []Diagnostic, error) {
	e.check()
	if hasError(e.diags) {
		return nil, e.diags, nil
	}

	out := e.global.Core.Clone()
	origins := make([]string, len(out.Edges))

	for _, p := range e.global.Promotes {
		local := e.locals[p.Map]
		if local == nil {
			continue
		}
		for _, edge := range local.Edges {
			expanded, origin := e.promoteEdge(p, local, edge)
			out.Edges = append(out.Edges, expanded)
			origins = append(origins, origin)
		}
	}

	x := &Expansion{Diagram: out, Origins: origins}
	x.sortEdges()
	return x, e.diags, nil
}

// promoteEdge lifts one local edge onto the global state the block was written
// in, and returns the comment that says where it came from.
func (e *expander) promoteEdge(p Promote, local *csdf.Diagram, edge csdf.Edge) (csdf.Edge, string) {
	absent := local.StartEdge.Dst
	src, dst := local.States[edge.Src], local.States[edge.Dst]

	var guard, post []string
	switch {
	case edge.Src == absent: // Creation: the instance starts to exist.
		guard = append(guard, fmt.Sprintf("%s ∉ dom %s", p.IDParam, p.Map))
		post = append(post, fmt.Sprintf("%s' = %s ∪ {%s ↦ 〈%s〉}", p.Map, p.Map, p.IDParam, dst.Name))

	case edge.Dst == absent: // Deletion: the instance stops existing.
		guard = append(guard,
			fmt.Sprintf("%s ∈ dom %s", p.IDParam, p.Map),
			fmt.Sprintf("%s(%s) ∈ 〈%s〉", p.Map, p.IDParam, src.Name))
		post = append(post, fmt.Sprintf("%s' = {%s} ⩤ %s", p.Map, p.IDParam, p.Map))

	default:
		guard = append(guard,
			fmt.Sprintf("%s ∈ dom %s", p.IDParam, p.Map),
			fmt.Sprintf("%s(%s) ∈ 〈%s〉", p.Map, p.IDParam, src.Name))
		post = append(post, fmt.Sprintf("%s' = %s ⊕ {%s ↦ 〈%s〉}", p.Map, p.Map, p.IDParam, dst.Name))
	}

	if !csdf.IsTrue(edge.Guard) {
		guard = append(guard, string(edge.Guard))
	}
	// The post of a deletion says something about an instance that is gone, so
	// it is dropped rather than carried into the expansion.
	if !csdf.IsTrue(edge.Post) && edge.Dst != absent {
		post = append(post, string(edge.Post))
	}
	post = append(post, e.frame(p)...)

	return csdf.Edge{
		Src:   p.In,
		Dst:   p.In,
		Event: promoteEvent(edge.Event, p.IDParam),
		Guard: csdf.Predicate(strings.Join(guard, " ∧ ")),
		Post:  csdf.Predicate(strings.Join(post, " ∧ ")),
	}, fmt.Sprintf("promote: %s 〈%s〉 → 〈%s〉", e.paths[p.Map], src.Name, dst.Name)
}

// frame says that the state variables this edge does not touch are unchanged.
// The map itself is not framed: ⊕, ∪ and ⩤ already say what happens to every
// key but the one the edge is about.
func (e *expander) frame(p Promote) []string {
	var clauses []string
	for _, v := range e.global.Core.States[p.In].Vars {
		if v.Name == p.Map {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s' = %s", v.Name, v.Name))
	}
	return clauses
}

// promoteEvent writes the instance ID in as the event's first argument. A tau is
// left alone: an internal event with an argument is an observable one, and the
// instance it happens to is implicitly existentially quantified.
func promoteEvent(event csdf.Event, id string) csdf.Event {
	if event == csdf.Tau {
		return event
	}
	if m := eventRe.FindStringSubmatch(string(event)); m != nil {
		if m[2] == "" {
			return csdf.Event(fmt.Sprintf("%s(%s)", m[1], id))
		}
		return csdf.Event(fmt.Sprintf("%s(%s, %s)", m[1], id, m[2]))
	}
	return csdf.Event(fmt.Sprintf("%s(%s)", strings.TrimSpace(string(event)), id))
}

// sortEdges puts the edges in the canonical order, carrying each origin comment
// along with the edge it belongs to.
func (x *Expansion) sortEdges() {
	order := make([]int, len(x.Diagram.Edges))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return csdf.CompareEdge(x.Diagram.Edges[a], x.Diagram.Edges[b])
	})

	edges := make([]csdf.Edge, len(order))
	origins := make([]string, len(order))
	for i, from := range order {
		edges[i] = x.Diagram.Edges[from]
		origins[i] = x.Origins[from]
	}
	x.Diagram.Edges = edges
	x.Origins = origins
}

// FileLoader resolves !include paths against base, the directory the global
// diagram was read from.
func FileLoader(base string) LoadFunc {
	return func(path string) (*csdf.Diagram, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		return csdf.LoadDiagram(path)
	}
}
