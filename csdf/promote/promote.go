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

// String prints the expanded diagram as PlantUML. With comments, every
// generated edge is preceded by a line comment naming the local edge it came
// from. Pairing the two is the reason to call this rather than to print the
// diagram and the origins separately.
func (r *Result) String(comments bool) string {
	if !comments {
		return r.Diagram.String()
	}
	return r.Diagram.StringWithEdgeComments(r.Origins)
}

// edgeWithOrigin keeps an edge and its origin together while the edges are
// sorted, which is the only reason Origins can be a plain parallel slice.
type edgeWithOrigin struct {
	Edge csdf.Edge
	// Generated tells an expanded edge from one the author wrote by hand. It is
	// not the same question as whether Origin is empty, even though the two
	// answers happen to agree: Origin is text for a reader.
	Generated bool
	Origin    string
}

// RunOptions is what Run needs beyond the diagram itself.
type RunOptions struct {
	// Templates renders the generated phrases. A nil value means the symbolic
	// ones (DefaultClauseTemplates).
	Templates *Templates
	// Werror makes a warning as fatal as an error.
	Werror bool
}

// Run expands the directives and judges the diagnostics: an error is fatal, and
// so is a warning under Werror. The diagnostics come back either way, since a
// caller reports all of them and prints the diagram only when nothing is fatal.
func Run(global *csdf.Diagram, load csdf.DiagramLoader, opts RunOptions) (*Result, []Diagnostic, error) {
	result, diags := Expand(global, load, Options{Templates: opts.Templates})

	if errs := Errors(diags); len(errs) > 0 {
		return result, diags, fmt.Errorf("%s in the promotion directives", pluralize(len(errs), "error"))
	}

	if opts.Werror {
		warnings := 0
		for _, diag := range diags {
			if diag.Severity == SeverityWarning {
				warnings++
			}
		}
		if warnings > 0 {
			return result, diags, fmt.Errorf("%s in the promotion directives (-Werror)", pluralize(warnings, "warning"))
		}
	}

	return result, diags, nil
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// Options is what an expansion can be told about how to render itself.
type Options struct {
	// Templates renders the generated phrases. A nil value means the default,
	// symbolic ones (DefaultTemplates).
	Templates *Templates
}

// Expand consumes every promotion directive of the diagram. It never fails
// outright: what goes wrong is reported as a diagnostic, and the caller decides
// what an error means (Errors).
func Expand(global *csdf.Diagram, load csdf.DiagramLoader, opts Options) (*Result, []Diagnostic) {
	var diags []Diagnostic

	render := newRenderer(opts.Templates)

	expanded := global.Clone()
	expanded.Promotes = nil
	expanded.Syncs = nil
	expanded.Constrains = nil

	edges := make([]edgeWithOrigin, 0, len(expanded.Edges))
	for _, edge := range expanded.Edges {
		edges = append(edges, edgeWithOrigin{Edge: edge})
	}

	promotions, order, resolveDiags := resolvePromotes(global, load)
	diags = append(diags, resolveDiags...)

	// A synced event is not the business of any one map, so it is left out of
	// the independent expansions and merged afterwards.
	synced := make(map[csdf.Var]map[string]bool)
	for _, sync := range global.Syncs {
		syncEdges, syncDiags := expandSync(global, sync, promotions, render)
		diags = append(diags, syncDiags...)
		edges = append(edges, syncEdges...)

		for _, target := range sync.Targets {
			if synced[target.Map] == nil {
				synced[target.Map] = make(map[string]bool)
			}
			synced[target.Map][sync.Event] = true
		}
	}

	for _, name := range order {
		promotion := promotions[name]
		for _, target := range promotion.Targets {
			for _, localEdge := range promotion.Local.Edges {
				eventName, _ := splitEvent(localEdge.Event)
				if synced[name][eventName] {
					continue
				}
				edges = append(edges, expandEdge(global, target, promotion.Promote, promotion.Local, localEdge, render))
			}
		}
	}

	diags = append(diags, render.diags...)
	diags = append(diags, checkSharedEvents(global, promotions, order)...)

	for _, constrain := range global.Constrains {
		diags = append(diags, applyConstrain(constrain, edges)...)
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

// resolvedPromote is a promote directive with everything it names looked up:
// the local diagram it promotes and the global states it is expanded into.
type resolvedPromote struct {
	Promote csdf.Promote
	Local   *csdf.Diagram
	Targets []csdf.StateID
}

// resolvePromotes checks and loads every promote directive, returning them by
// map together with the order they were written in.
func resolvePromotes(global *csdf.Diagram, load csdf.DiagramLoader) (map[csdf.Var]*resolvedPromote, []csdf.Var, []Diagnostic) {
	var diags []Diagnostic
	promotions := make(map[csdf.Var]*resolvedPromote, len(global.Promotes))
	var order []csdf.Var

	for _, promotion := range global.Promotes {
		if earlier, ok := promotions[promotion.Map]; ok {
			// Two promotions of one map would each frame the other out, so
			// neither expansion says what the map does.
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     promotion.Line,
				Message:  fmt.Sprintf("promote via %q: the map is already promoted at line %d", promotion.Map, earlier.Promote.Line),
			})
			continue
		}

		resolved, promoteDiags := resolvePromote(global, promotion, load)
		diags = append(diags, promoteDiags...)
		if resolved == nil {
			continue
		}
		promotions[promotion.Map] = resolved
		order = append(order, promotion.Map)
	}

	return promotions, order, diags
}

func resolvePromote(global *csdf.Diagram, promotion csdf.Promote, load csdf.DiagramLoader) (*resolvedPromote, []Diagnostic) {
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

	for _, state := range csdf.SortedStates(global.States) {
		if !hasVar(state.State, promotion.Map) || slices.Contains(valid, state.ID) {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityInfo,
			Line:     promotion.Line,
			Message: fmt.Sprintf(
				"promote via %q: the state %q holds the map but is not in the in clause, so the map is frozen there",
				promotion.Map, state.ID),
		})
	}

	if len(valid) > 0 {
		// The type of a state variable is free text, so the most that can be
		// said is that it does not mention the type being promoted.
		if varType := varTypeOf(global.States[valid[0]], promotion.Map); varType != "" && !strings.Contains(varType, promotion.Type) {
			diags = append(diags, Diagnostic{
				Severity: SeverityWarning,
				Line:     promotion.Line,
				Message: fmt.Sprintf(
					"promote as %s: the type of %q is %q, which does not mention %s",
					promotion.Type, promotion.Map, varType, promotion.Type),
			})
		}
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

	return &resolvedPromote{Promote: promotion, Local: local, Targets: valid}, diags
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

	absentID := local.StartEdge.Dst
	for _, localEdge := range local.Edges {
		if localEdge.Dst != absentID || localEdge.Src == absentID || csdf.IsTrue(localEdge.Post) {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityWarning,
			Line:     promotion.Line,
			Message: fmt.Sprintf(
				"promote %s: the edge 〈%s〉 → 〈%s〉 deletes the instance, so its post %q is discarded",
				promotion.Path, stateName(local, localEdge.Src), stateName(local, localEdge.Dst), localEdge.Post),
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

// edgeParts is one local edge seen through its promotion: the clauses it
// contributes to the guard and the post of a global edge, and the pieces its
// event is rebuilt from. A sync merges the parts of several local edges into
// one global edge, so they are built before an edge is.
type edgeParts struct {
	Guard     []string
	Post      []string
	EventName string
	Args      []string
	Origin    string
}

func promoteEdgeParts(promotion csdf.Promote, local *csdf.Diagram, localEdge csdf.Edge, render *renderer) edgeParts {
	absent := local.StartEdge.Dst
	creates := localEdge.Src == absent
	deletes := localEdge.Dst == absent

	data := clauseData{
		Map:   promotion.Map,
		ID:    promotion.IDParam,
		Src:   stateName(local, localEdge.Src),
		Dst:   stateName(local, localEdge.Dst),
		Guard: localEdge.Guard,
		Post:  localEdge.Post,
	}

	var guard, post []string

	if creates {
		guard = append(guard, render.clause("absent", data))
	} else {
		guard = append(guard, render.clause("exists", data), render.clause("at", data))
	}
	if !csdf.IsTrue(localEdge.Guard) {
		guard = append(guard, string(localEdge.Guard))
	}

	switch {
	case creates && deletes:
		// An event that leaves the instance as absent as it was.
		post = append(post, render.clause("keep", data))
	case deletes:
		post = append(post, render.clause("delete", data))
	case creates:
		post = append(post, render.clause("insert", data))
	default:
		post = append(post, render.clause("update", data))
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

	name, args := splitEvent(localEdge.Event)
	return edgeParts{
		Guard:     guard,
		Post:      post,
		EventName: name,
		Args:      args,
		Origin:    fmt.Sprintf("%s 〈%s〉 → 〈%s〉", promotion.Path, data.Src, data.Dst),
	}
}

func expandEdge(global *csdf.Diagram, target csdf.StateID, promotion csdf.Promote, local *csdf.Diagram, localEdge csdf.Edge, render *renderer) edgeWithOrigin {
	parts := promoteEdgeParts(promotion, local, localEdge, render)
	post := append(parts.Post, frame(global, target, render, promotion.Map)...)

	return edgeWithOrigin{
		Generated: true,
		Edge: csdf.Edge{
			Src:   target,
			Dst:   target,
			Event: promotedEvent(localEdge.Event, parts.EventName, append([]string{promotion.IDParam}, parts.Args...)),
			Guard: csdf.Predicate(strings.Join(parts.Guard, Conjunction)),
			Post:  csdf.Predicate(strings.Join(post, Conjunction)),
		},
		Origin: "promote: " + parts.Origin,
	}
}

// frame says that the other state variables of the global state do not move.
// The maps being promoted need no clause of their own: ⊕, ∪ and ⩤ already say
// what happens to every key but the one at hand.
func frame(global *csdf.Diagram, target csdf.StateID, render *renderer, promoted ...csdf.Var) []string {
	var others []csdf.Var
	for _, v := range global.States[target].Vars {
		if !slices.Contains(promoted, v.Name) {
			others = append(others, v.Name)
		}
	}

	clauses := make([]string, 0, len(others))
	for _, other := range others {
		clauses = append(clauses, render.clause("unchanged", clauseData{Other: other, OtherMaps: others}))
	}
	return clauses
}

// splitEvent separates a local event into its name and its argument names.
func splitEvent(event csdf.Event) (string, []string) {
	name, args, found := strings.Cut(string(event), "(")
	name = strings.TrimSpace(name)
	if !found {
		return name, nil
	}
	args = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(args), ")"))
	if args == "" {
		return name, nil
	}
	parsed := strings.Split(args, ",")
	for i, arg := range parsed {
		parsed[i] = strings.TrimSpace(arg)
	}
	return name, parsed
}

// promotedEvent rebuilds an event with the instance ids in front of its
// arguments. τ is left alone: giving it an id would make it observable, which
// is the opposite of what hiding an event means.
func promotedEvent(local csdf.Event, name string, args []string) csdf.Event {
	if local == csdf.Tau {
		return local
	}
	return csdf.Event(fmt.Sprintf("%s(%s)", name, strings.Join(args, ", ")))
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

// expandSync merges the edges the synced event contributes to each of its maps
// into one global edge per combination, so that the instances take the event
// together. The combinations are the cartesian product of the per-map edge
// groups, because a guard on one side may pick out any state on the other.
func expandSync(global *csdf.Diagram, sync csdf.Sync, promotions map[csdf.Var]*resolvedPromote, render *renderer) ([]edgeWithOrigin, []Diagnostic) {
	var diags []Diagnostic

	if sync.Event == string(csdf.Tau) {
		return nil, []Diagnostic{{
			Severity: SeverityError,
			Line:     sync.Line,
			Message:  fmt.Sprintf("sync %s: an internal event cannot be synchronised", sync.Event),
		}}
	}

	var targets []csdf.StateID
	groups := make([][]edgeParts, 0, len(sync.Targets))
	ok := true

	for i, ref := range sync.Targets {
		if slices.ContainsFunc(sync.Targets[:i], func(earlier csdf.MapRef) bool { return earlier.Map == ref.Map }) {
			// Both sides would frame the whole map, so the two updates could
			// only be satisfied by one instance being the other.
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     sync.Line,
				Message:  fmt.Sprintf("sync %s: the map %q is synced with itself; one edge cannot say twice what a map becomes", sync.Event, ref.Map),
			})
			ok = false
			continue
		}

		promotion, found := promotions[ref.Map]
		if !found {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     sync.Line,
				Message:  fmt.Sprintf("sync %s: the map %q is not promoted", sync.Event, ref.Map),
			})
			ok = false
			continue
		}

		// The sync directive names the instance ids, so its parameter is the
		// one the promoted predicates must speak of.
		renamed := promotion.Promote
		renamed.IDParam = ref.Param

		var group []edgeParts
		for _, localEdge := range promotion.Local.Edges {
			if name, _ := splitEvent(localEdge.Event); name == sync.Event {
				group = append(group, promoteEdgeParts(renamed, promotion.Local, localEdge, render))
			}
		}
		if len(group) == 0 {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     sync.Line,
				Message:  fmt.Sprintf("sync %s: %s has no such event", sync.Event, promotion.Promote.Path),
			})
			ok = false
			continue
		}
		groups = append(groups, group)

		if i == 0 {
			targets = slices.Clone(promotion.Targets)
			continue
		}
		targets = slices.DeleteFunc(targets, func(target csdf.StateID) bool {
			return !slices.Contains(promotion.Targets, target)
		})
	}

	if !ok {
		return nil, diags
	}
	if len(targets) == 0 {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Line:     sync.Line,
			Message:  fmt.Sprintf("sync %s: the promotions of its maps share no state, so the event can never happen", sync.Event),
		})
		return nil, diags
	}

	promoted := make([]csdf.Var, 0, len(sync.Targets))
	for _, ref := range sync.Targets {
		promoted = append(promoted, ref.Map)
	}

	var edges []edgeWithOrigin
	for _, combination := range product(groups) {
		args := make([]string, 0, len(sync.Targets))
		for _, ref := range sync.Targets {
			args = appendUnique(args, ref.Param)
		}
		var guard, post, origins []string
		for _, part := range combination {
			guard = append(guard, part.Guard...)
			post = append(post, part.Post...)
			origins = append(origins, part.Origin)
			// Arguments of the same name are one argument: the instances agree
			// on the value the shared event carries.
			for _, arg := range part.Args {
				args = appendUnique(args, arg)
			}
		}

		for _, target := range targets {
			edges = append(edges, edgeWithOrigin{
				Generated: true,
				Edge: csdf.Edge{
					Src:   target,
					Dst:   target,
					Event: csdf.Event(fmt.Sprintf("%s(%s)", sync.Event, strings.Join(args, ", "))),
					Guard: csdf.Predicate(strings.Join(guard, Conjunction)),
					Post:  csdf.Predicate(strings.Join(append(slices.Clone(post), frame(global, target, render, promoted...)...), Conjunction)),
				},
				Origin: fmt.Sprintf("sync: %s %s", sync.Event, strings.Join(origins, " + ")),
			})
		}
	}
	return edges, diags
}

// product enumerates one choice from each group, in the order the groups were
// given.
func product(groups [][]edgeParts) [][]edgeParts {
	combinations := [][]edgeParts{{}}
	for _, group := range groups {
		next := make([][]edgeParts, 0, len(combinations)*len(group))
		for _, combination := range combinations {
			for _, part := range group {
				next = append(next, append(slices.Clone(combination), part))
			}
		}
		combinations = next
	}
	return combinations
}

func appendUnique(args []string, arg string) []string {
	if slices.Contains(args, arg) {
		return args
	}
	return append(args, arg)
}

// applyConstrain conjoins the constraint onto every expanded edge whose event
// matches it in name and arity. Only expanded edges are touched: an edge the
// author wrote by hand already says everything they meant it to.
func applyConstrain(constrain csdf.Constrain, edges []edgeWithOrigin) []Diagnostic {
	matched := 0
	for i, edge := range edges {
		if !edge.Generated {
			continue
		}
		name, args := splitEvent(edge.Edge.Event)
		if name != constrain.Event || len(args) != len(constrain.Params) {
			continue
		}
		edges[i].Edge.Guard = csdf.Predicate(fmt.Sprintf("%s%s%s", edge.Edge.Guard, Conjunction, constrain.Guard))
		matched++
	}

	if matched > 0 && !mentionsAny(string(constrain.Guard), constrain.Params) {
		// The guard is opaque, so the most that can be said is that it never
		// names what it was given to talk about.
		return []Diagnostic{{
			Severity: SeverityWarning,
			Line:     constrain.Line,
			Message: fmt.Sprintf(
				"constrain %s/%d: the guard mentions none of its parameters (%s)",
				constrain.Event, len(constrain.Params), strings.Join(constrain.Params, ", ")),
		}}
	}

	if matched == 0 {
		return []Diagnostic{{
			Severity: SeverityError,
			Line:     constrain.Line,
			Message: fmt.Sprintf(
				"constrain %s/%d: no expanded edge carries that event with that many arguments",
				constrain.Event, len(constrain.Params)),
		}}
	}
	return nil
}

// checkSharedEvents reads back the local event names that appear in more than
// one local diagram without a sync directive. Sharing a name and yet taking the
// event independently is legitimate, but it is more often an oversight.
func checkSharedEvents(global *csdf.Diagram, promotions map[csdf.Var]*resolvedPromote, order []csdf.Var) []Diagnostic {
	synced := make(map[string]bool, len(global.Syncs))
	for _, sync := range global.Syncs {
		synced[sync.Event] = true
	}

	var events []string
	paths := make(map[string][]string)
	for _, name := range order {
		promotion := promotions[name]
		seen := make(map[string]bool)
		for _, localEdge := range promotion.Local.Edges {
			eventName, _ := splitEvent(localEdge.Event)
			if eventName == string(csdf.Tau) || seen[eventName] {
				continue
			}
			seen[eventName] = true
			if len(paths[eventName]) == 0 {
				events = append(events, eventName)
			}
			paths[eventName] = append(paths[eventName], promotion.Promote.Path)
		}
	}
	slices.Sort(events)

	var diags []Diagnostic
	for _, event := range events {
		if synced[event] || len(paths[event]) < 2 {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"the event %s is in %s but is not synced; the instances take it independently",
				event, joinAnd(paths[event])),
		})
	}
	return diags
}

func joinAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func mentionsAny(text string, names []string) bool {
	for _, name := range names {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

func varTypeOf(state csdf.State, name csdf.Var) string {
	for _, v := range state.Vars {
		if v.Name == name {
			return v.Type
		}
	}
	return ""
}
