// Package promote expands the promotion directives of a global diagram
// (docs/PROMOTION.md) into ordinary edges, so that the tools that analyse a
// Composable State Diagram never have to know about promotion.
package promote

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Loader reads the local diagram a promote directive names. The path is taken
// verbatim from the directive, so resolving it against a base directory is the
// caller's business.
type Loader func(path string) (*csdf.Diagram, error)

type Severity int

const (
	// SeverityError marks a diagram the expansion cannot be trusted for.
	SeverityError Severity = iota
	// SeverityWarning marks something the author probably did not mean.
	SeverityWarning
	// SeverityInfo reads back a consequence of the directives that is easy to
	// overlook but perfectly legitimate.
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return fmt.Sprintf("Severity(%d)", int(s))
	}
}

// Diagnostic is one finding of the structural check that expansion performs on
// the way. Line is the 1-based line of the directive it came from, or 0 when it
// belongs to no single directive.
type Diagnostic struct {
	Severity Severity
	Line     int
	Message  string
}

func (d Diagnostic) String() string {
	if d.Line == 0 {
		return fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: line %d: %s", d.Severity, d.Line, d.Message)
}

// Errors returns the diagnostics that make the expansion untrustworthy.
func Errors(diags []Diagnostic) []Diagnostic {
	var errs []Diagnostic
	for _, diag := range diags {
		if diag.Severity == SeverityError {
			errs = append(errs, diag)
		}
	}
	return errs
}

// Result is an expanded diagram together with, for each of its edges, the local
// edge it came from. Origins is aligned with Diagram.Edges and holds "" for an
// edge the author wrote by hand.
type Result struct {
	Diagram *csdf.Diagram
	Origins []string
}

// edgeWithOrigin keeps an edge and its origin together while the edges are
// sorted, which is the only reason Origins can be a plain parallel slice.
type edgeWithOrigin struct {
	Edge   csdf.Edge
	Origin string
}

// Expand consumes every promotion directive of the diagram. It never fails
// outright: what goes wrong is reported as a diagnostic, and the caller decides
// what an error means (Errors).
func Expand(global *csdf.Diagram, load Loader) (*Result, []Diagnostic) {
	var diags []Diagnostic

	expanded := global.Clone()
	expanded.Promotes = nil
	expanded.Syncs = nil
	expanded.Constrains = nil

	edges := make([]edgeWithOrigin, 0, len(expanded.Edges))
	for _, edge := range expanded.Edges {
		edges = append(edges, edgeWithOrigin{Edge: edge})
	}

	for _, promotion := range global.Promotes {
		promoted, promoteDiags := expandPromote(global, promotion, load)
		diags = append(diags, promoteDiags...)
		edges = append(edges, promoted...)
	}

	slices.SortFunc(edges, func(a, b edgeWithOrigin) int {
		return csdf.CompareEdge(a.Edge, b.Edge)
	})

	expanded.Edges = make([]csdf.Edge, 0, len(edges))
	origins := make([]string, 0, len(edges))
	for _, edge := range edges {
		expanded.Edges = append(expanded.Edges, edge.Edge)
		origins = append(origins, edge.Origin)
	}

	return &Result{Diagram: expanded, Origins: origins}, diags
}

func expandPromote(global *csdf.Diagram, promotion csdf.Promote, load Loader) ([]edgeWithOrigin, []Diagnostic) {
	var diags []Diagnostic

	targets := promotion.In
	if len(targets) == 0 {
		targets = []csdf.StateID{global.StartEdge.Dst}
	}

	var valid []csdf.StateID
	for _, target := range targets {
		state, ok := global.States[target]
		if !ok {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     promotion.Line,
				Message:  fmt.Sprintf("promote into %q: no such state in this diagram", target),
			})
			continue
		}
		if !hasVar(state, promotion.Map) {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     promotion.Line,
				Message:  fmt.Sprintf("promote into %q: the state has no state variable %q to promote through", target, promotion.Map),
			})
			continue
		}
		valid = append(valid, target)
	}

	local, err := load(promotion.Path)
	if err != nil {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Line:     promotion.Line,
			Message:  fmt.Sprintf("promote %s: %v", promotion.Path, err),
		})
		return nil, diags
	}
	diags = append(diags, checkLocal(promotion, local)...)

	var edges []edgeWithOrigin
	for _, target := range valid {
		for _, localEdge := range local.Edges {
			edges = append(edges, expandEdge(global, target, promotion, local, localEdge))
		}
	}
	return edges, diags
}

// checkLocal reports the ways a local diagram can fail the promotion contract:
// its start state stands for "no such instance", so it holds nothing, and an
// instance that ends is one that is deleted, not one that reaches [*].
func checkLocal(promotion csdf.Promote, local *csdf.Diagram) []Diagnostic {
	var diags []Diagnostic

	absent := local.States[local.StartEdge.Dst]
	if len(absent.Vars) > 0 {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Line:     promotion.Line,
			Message: fmt.Sprintf(
				"promote %s: the start state %q means that no such instance exists, so it must have no state variables",
				promotion.Path, local.StartEdge.Dst),
		})
	}

	if local.EndEdge != nil {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Line:     promotion.Line,
			Message: fmt.Sprintf(
				"promote %s: an end edge cannot be promoted; write a state with no outgoing edge instead",
				promotion.Path),
		})
	}

	return diags
}

func expandEdge(global *csdf.Diagram, target csdf.StateID, promotion csdf.Promote, local *csdf.Diagram, localEdge csdf.Edge) edgeWithOrigin {
	absent := local.StartEdge.Dst
	creates := localEdge.Src == absent
	deletes := localEdge.Dst == absent

	var guard, post []string

	if creates {
		guard = append(guard, fmt.Sprintf("%s ∉ dom %s", promotion.IDParam, promotion.Map))
	} else {
		guard = append(guard,
			fmt.Sprintf("%s ∈ dom %s", promotion.IDParam, promotion.Map),
			fmt.Sprintf("%s(%s) ∈ 〈%s〉", promotion.Map, promotion.IDParam, stateName(local, localEdge.Src)))
	}
	if !csdf.IsTrue(localEdge.Guard) {
		guard = append(guard, string(localEdge.Guard))
	}

	switch {
	case creates && deletes:
		// An event that leaves the instance as absent as it was.
		post = append(post, fmt.Sprintf("%s' = %s", promotion.Map, promotion.Map))
	case deletes:
		post = append(post, fmt.Sprintf("%s' = {%s} ⩤ %s", promotion.Map, promotion.IDParam, promotion.Map))
	case creates:
		post = append(post, fmt.Sprintf("%s' = %s ∪ {%s ↦ 〈%s〉}", promotion.Map, promotion.Map, promotion.IDParam, stateName(local, localEdge.Dst)))
	default:
		post = append(post, fmt.Sprintf("%s' = %s ⊕ {%s ↦ 〈%s〉}", promotion.Map, promotion.Map, promotion.IDParam, stateName(local, localEdge.Dst)))
	}
	// Deleting an instance discards its local post: the absent state holds
	// nothing for the post to talk about.
	if !deletes && !csdf.IsTrue(localEdge.Post) {
		post = append(post, string(localEdge.Post))
	}
	// The local start edge is the initial value of a fresh instance.
	if creates && !deletes && !csdf.IsTrue(local.StartEdge.Post) {
		post = append(post, string(local.StartEdge.Post))
	}
	post = append(post, frame(global, target, promotion.Map)...)

	return edgeWithOrigin{
		Edge: csdf.Edge{
			Src:   target,
			Dst:   target,
			Event: promoteEvent(localEdge.Event, promotion.IDParam),
			Guard: csdf.Predicate(strings.Join(guard, " ∧ ")),
			Post:  csdf.Predicate(strings.Join(post, " ∧ ")),
		},
		Origin: fmt.Sprintf("promote: %s 〈%s〉 → 〈%s〉", promotion.Path, stateName(local, localEdge.Src), stateName(local, localEdge.Dst)),
	}
}

// frame says that the other state variables of the global state do not move.
// The map being promoted needs no clause of its own: ⊕, ∪ and ⩤ already say
// what happens to every key but the one at hand.
func frame(global *csdf.Diagram, target csdf.StateID, promoted csdf.Var) []string {
	var clauses []string
	for _, v := range global.States[target].Vars {
		if v.Name == promoted {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s' = %s", v.Name, v.Name))
	}
	return clauses
}

// promoteEvent prefixes the instance id to the arguments of a local event. τ is
// left alone: giving it an id would make it observable, which is the opposite
// of what hiding an event means.
func promoteEvent(event csdf.Event, idParam string) csdf.Event {
	if event == csdf.Tau {
		return event
	}

	name, args, found := strings.Cut(string(event), "(")
	if !found {
		return csdf.Event(fmt.Sprintf("%s(%s)", strings.TrimSpace(string(event)), idParam))
	}
	args = strings.TrimSuffix(strings.TrimSpace(args), ")")
	if strings.TrimSpace(args) == "" {
		return csdf.Event(fmt.Sprintf("%s(%s)", strings.TrimSpace(name), idParam))
	}
	return csdf.Event(fmt.Sprintf("%s(%s, %s)", strings.TrimSpace(name), idParam, args))
}

func stateName(d *csdf.Diagram, id csdf.StateID) string {
	if state, ok := d.States[id]; ok && state.Name != "" {
		return state.Name
	}
	return string(id)
}

func hasVar(state csdf.State, name csdf.Var) bool {
	for _, v := range state.Vars {
		if v.Name == name {
			return true
		}
	}
	return false
}
