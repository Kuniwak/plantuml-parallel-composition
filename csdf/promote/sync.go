package promote

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// syncPlan is one sync directive worked out against the promotions: the states
// the merged edges land on, and the block of each target map in each of them.
type syncPlan struct {
	sync   Sync
	states []csdf.StateID
	// blocks[state][i] is the block of the i-th target in that state.
	blocks map[csdf.StateID][]Promote
}

// planSyncs reads every sync directive against the promotions and reports what
// cannot mean anything. A sync merges the edges its maps contribute to one
// event, so it needs every map promoted, the event present on every side, and a
// state where all of them move.
func (e *expander) planSyncs() {
	for _, s := range e.global.Syncs {
		plan := syncPlan{sync: s, blocks: map[csdf.StateID][]Promote{}}

		if s.Event == string(csdf.Tau) {
			e.errorf(s.Line, "tau cannot be synced; an internal event two families take together is an observable one")
			continue
		}

		ok := true
		var states []csdf.StateID
		named := map[csdf.Var]bool{}
		for i, t := range s.Targets {
			// One edge updates one key of one map. Two targets on one map would
			// put two assignments to that map in one post, which no state can
			// satisfy unless the two IDs are equal.
			if named[t.Map] {
				e.errorf(s.Line, "this sync names %q twice; one event cannot move two instances of one map at once", t.Map)
				ok = false
				continue
			}
			named[t.Map] = true

			in := e.statesOf(t.Map)
			if len(in) == 0 {
				e.errorf(s.Line, "%q is not promoted anywhere", t.Map)
				ok = false
				continue
			}
			edges := edgesOn(e.locals[t.Map], s.Event)
			if len(edges) == 0 {
				e.errorf(s.Line, "%s has no edge on %q", e.paths[t.Map], s.Event)
				ok = false
				continue
			}
			// Every combination of the sides becomes one merged event, so sides
			// that disagree on their arguments make the merged event's arity
			// depend on which combination it is - and a constrain matches on
			// arity.
			if !sameArguments(edges) {
				e.warnf(s.Line, "the edges of %q on %q do not all take the same arguments, so the merged event does not either", t.Map, s.Event)
			}
			if i == 0 {
				states = in
				continue
			}
			states = intersect(states, in)
		}
		if !ok {
			continue
		}
		if len(states) == 0 {
			e.errorf(s.Line, "the maps of this sync never move in the same state, so the event they take together can never happen")
			continue
		}

		plan.states = states
		for _, g := range states {
			for _, t := range s.Targets {
				plan.blocks[g] = append(plan.blocks[g], e.blockOf(t.Map, g))
			}
		}
		e.plans = append(e.plans, plan)

		// The event is the sync's from here on: the maps do not expand it on
		// their own in the states the merged edges land on.
		for _, t := range s.Targets {
			for _, g := range states {
				e.owned[ownership{g, t.Map, s.Event}] = true
			}
		}
	}
}

// ownership marks an event as belonging to a sync in one state, so the map's own
// expansion leaves it alone there.
type ownership struct {
	state csdf.StateID
	m     csdf.Var
	event string
}

// mergeSyncs emits one edge per combination of the local edges its maps
// contribute: a guard on one side can pick out any state on the other, so the
// product is what the two families can do together.
func (e *expander) mergeSyncs(edges []ExpandedEdge) []ExpandedEdge {
	for _, plan := range e.plans {
		for _, g := range plan.states {
			blocks := plan.blocks[g]

			choices := make([][]csdf.Edge, len(blocks))
			for i, b := range blocks {
				choices[i] = edgesOn(e.locals[b.Map], plan.sync.Event)
			}

			for _, combination := range product(choices) {
				parts := make([]edgeParts, len(blocks))
				moved := make([]csdf.Var, len(blocks))
				for i, edge := range combination {
					parts[i] = e.parts(blocks[i], edge, plan.sync.Targets[i].Param)
					moved[i] = blocks[i].Map
				}
				edges = append(edges, ExpandedEdge{
					Edge:   e.compose(g, parts, moved...),
					Origin: e.syncOrigin(plan, blocks, parts),
				})
			}
		}
	}
	return edges
}

func (e *expander) syncOrigin(plan syncPlan, blocks []Promote, parts []edgeParts) string {
	sides := make([]string, len(parts))
	for i, part := range parts {
		sides[i] = fmt.Sprintf("(%s 〈%s〉 → 〈%s〉)", e.paths[blocks[i].Map], part.src, part.dst)
	}
	return fmt.Sprintf("sync: %s %s", plan.sync.Event, strings.Join(sides, " × "))
}

// constrain conjoins each constrain guard onto every generated edge that matches
// the event in name and arity. A hand-written edge is left alone: it already
// says everything it was meant to.
func (e *expander) constrain(edges []ExpandedEdge) {
	for _, c := range e.global.Constrains {
		matched := 0
		for i := range edges {
			if edges[i].Origin == "" {
				continue
			}
			name, args := splitEvent(edges[i].Edge.Event)
			if name != c.Event || len(args) != len(c.Params) {
				continue
			}
			matched++
			for _, param := range c.Params {
				if !slices.Contains(args, param) {
					e.warnf(c.Line, "%s has no argument named %q; the guard says nothing about it", edges[i].Edge.Event, param)
				}
			}

			guard := edges[i].Edge.Guard
			if csdf.IsTrue(guard) {
				edges[i].Edge.Guard = c.Guard
				continue
			}
			edges[i].Edge.Guard = csdf.Predicate(e.templates.join([]string{string(guard), string(c.Guard)}))
		}
		if matched == 0 {
			e.errorf(c.Line, "no expanded edge is %q with %d argument%s", c.Event, len(c.Params), plural(len(c.Params)))
		}
	}
}

// statesOf reports the states a map is promoted in, in source order.
func (e *expander) statesOf(m csdf.Var) []csdf.StateID {
	var states []csdf.StateID
	for _, p := range e.global.Promotes {
		if p.Map == m && !slices.Contains(states, p.In) {
			states = append(states, p.In)
		}
	}
	return states
}

// blockOf returns the block that promotes m in state g.
func (e *expander) blockOf(m csdf.Var, g csdf.StateID) Promote {
	for _, p := range e.global.Promotes {
		if p.Map == m && p.In == g {
			return p
		}
	}
	return Promote{}
}

// edgesOn returns the local edges carrying the named event, in source order.
func edgesOn(local *csdf.Diagram, event string) []csdf.Edge {
	var edges []csdf.Edge
	for _, edge := range local.Edges {
		if name, _ := splitEvent(edge.Event); name == event {
			edges = append(edges, edge)
		}
	}
	return edges
}

// sameArguments reports whether every edge takes the same argument list.
func sameArguments(edges []csdf.Edge) bool {
	_, first := splitEvent(edges[0].Event)
	for _, edge := range edges[1:] {
		if _, args := splitEvent(edge.Event); !slices.Equal(args, first) {
			return false
		}
	}
	return true
}

// product returns every way of choosing one edge from each side.
func product(choices [][]csdf.Edge) [][]csdf.Edge {
	out := [][]csdf.Edge{{}}
	for _, side := range choices {
		next := make([][]csdf.Edge, 0, len(out)*len(side))
		for _, prefix := range out {
			for _, edge := range side {
				next = append(next, append(append([]csdf.Edge{}, prefix...), edge))
			}
		}
		out = next
	}
	return out
}

func intersect(a, b []csdf.StateID) []csdf.StateID {
	var out []csdf.StateID
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
