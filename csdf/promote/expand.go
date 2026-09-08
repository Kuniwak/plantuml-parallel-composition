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
	e := &expander{global: g, load: load, locals: map[csdf.Var]*csdf.Diagram{}, paths: map[csdf.Var]string{}, owned: map[ownership]bool{}}
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
	// plans is one worked-out sync per directive, and owned marks the events a
	// sync took over so that the maps do not expand them on their own.
	plans []syncPlan
	owned map[ownership]bool
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
			if name, _ := splitEvent(edge.Event); e.owned[ownership{p.In, p.Map, name}] {
				continue
			}
			expanded, origin := e.promoteEdge(p, edge)
			out.Edges = append(out.Edges, expanded)
			origins = append(origins, origin)
		}
	}

	e.mergeSyncs(out, &origins)
	e.constrain(out, origins)
	if hasError(e.diags) {
		return nil, e.diags, nil
	}

	x := &Expansion{Diagram: out, Origins: origins}
	x.sortEdges()
	return x, e.diags, nil
}

// edgeParts is one local edge promoted through one map: the clauses it
// contributes to a global edge, and what it needs to name itself.
type edgeParts struct {
	guard []string
	post  []string
	name  string   // The event name, without its arguments.
	args  []string // The event's own arguments, after the instance ID.
	id    string   // The parameter the instance ID is written as.
	tau   bool
	src   string // The local source state's name, for the origin comment.
	dst   string // The local destination state's name.
}

// parts promotes one local edge through map p, writing the instance ID as id.
// The clauses are not joined here: a synced edge is the conjunction of the parts
// of one local edge per map, and a plain one is the conjunction of just this.
func (e *expander) parts(p Promote, edge csdf.Edge, id string) edgeParts {
	local := e.locals[p.Map]
	absent := local.StartEdge.Dst
	src, dst := local.States[edge.Src], local.States[edge.Dst]

	parts := edgeParts{src: src.Name, dst: dst.Name, id: id, tau: edge.Event == csdf.Tau}
	parts.name, parts.args = splitEvent(edge.Event)

	switch {
	case edge.Src == absent: // Creation: the instance starts to exist.
		parts.guard = append(parts.guard, fmt.Sprintf("%s ∉ dom %s", id, p.Map))
		parts.post = append(parts.post, fmt.Sprintf("%s' = %s ∪ {%s ↦ 〈%s〉}", p.Map, p.Map, id, dst.Name))

	case edge.Dst == absent: // Deletion: the instance stops existing.
		parts.guard = append(parts.guard,
			fmt.Sprintf("%s ∈ dom %s", id, p.Map),
			fmt.Sprintf("%s(%s) ∈ 〈%s〉", p.Map, id, src.Name))
		parts.post = append(parts.post, fmt.Sprintf("%s' = {%s} ⩤ %s", p.Map, id, p.Map))

	default:
		parts.guard = append(parts.guard,
			fmt.Sprintf("%s ∈ dom %s", id, p.Map),
			fmt.Sprintf("%s(%s) ∈ 〈%s〉", p.Map, id, src.Name))
		parts.post = append(parts.post, fmt.Sprintf("%s' = %s ⊕ {%s ↦ 〈%s〉}", p.Map, p.Map, id, dst.Name))
	}

	if !csdf.IsTrue(edge.Guard) {
		parts.guard = append(parts.guard, string(edge.Guard))
	}
	// The post of a deletion says something about an instance that is gone, so
	// it is dropped rather than carried into the expansion.
	if !csdf.IsTrue(edge.Post) && edge.Dst != absent {
		parts.post = append(parts.post, string(edge.Post))
	}
	return parts
}

// promoteEdge lifts one local edge onto the state its block was written in.
func (e *expander) promoteEdge(p Promote, edge csdf.Edge) (csdf.Edge, string) {
	parts := e.parts(p, edge, p.IDParam)
	out := e.compose(p.In, []edgeParts{parts}, p.Map)
	return out, fmt.Sprintf("promote: %s 〈%s〉 → 〈%s〉", e.paths[p.Map], parts.src, parts.dst)
}

// compose joins the parts of one or more local edges into one global self-loop
// on state g, framing every state variable of g but the maps the parts moved.
func (e *expander) compose(g csdf.StateID, parts []edgeParts, moved ...csdf.Var) csdf.Edge {
	var guard, post, ids, args []string
	tau := false

	for _, part := range parts {
		guard = append(guard, part.guard...)
		post = append(post, part.post...)
		tau = tau || part.tau
		if !part.tau {
			ids = appendUnique(ids, part.id)
			args = appendUnique(args, part.args...)
		}
	}
	post = append(post, e.frame(g, moved)...)

	event := csdf.Tau
	if !tau {
		event = csdf.Event(fmt.Sprintf("%s(%s)", parts[0].name, strings.Join(append(ids, args...), ", ")))
	}

	return csdf.Edge{
		Src:   g,
		Dst:   g,
		Event: event,
		Guard: csdf.Predicate(strings.Join(guard, " ∧ ")),
		Post:  csdf.Predicate(strings.Join(post, " ∧ ")),
	}
}

// splitEvent takes a local event apart into its name and its arguments. An event
// is a free string, so this is a convention rather than grammar: what is not of
// this shape is a name with no arguments.
func splitEvent(event csdf.Event) (string, []string) {
	m := eventRe.FindStringSubmatch(string(event))
	if m == nil {
		return strings.TrimSpace(string(event)), nil
	}
	if m[2] == "" {
		return m[1], nil
	}
	return m[1], splitTrimmed(m[2])
}

// appendUnique appends the values that are not in dst yet. Arguments of one name
// mean one thing, which is how a synced event ends up with one of each.
func appendUnique(dst []string, values ...string) []string {
	for _, v := range values {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

// frame says that the state variables this edge does not touch are unchanged.
// The maps it moved are not framed: ⊕, ∪ and ⩤ already say what happens to every
// key but the one the edge is about.
func (e *expander) frame(g csdf.StateID, moved []csdf.Var) []string {
	var clauses []string
	for _, v := range e.global.Core.States[g].Vars {
		if slices.Contains(moved, v.Name) {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s' = %s", v.Name, v.Name))
	}
	return clauses
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
