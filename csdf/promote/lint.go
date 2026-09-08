package promote

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// check reads the promotions for everything that would make the expansion
// unsound, and loads the local diagram of every map along the way. An expansion
// runs only when nothing here is an error.
func (e *expander) check() {
	e.checkBlocks()
	e.loadLocals()
	e.checkLocals()
	if HasError(e.diags) {
		// The syncs are read against the local diagrams, which are not all
		// there when a block is in error.
		return
	}
	e.planSyncs()
	e.warn()
}

// checkBlocks reads the <<promote>> blocks against each other and against the
// states they were written in.
func (e *expander) checkBlocks() {
	includers := map[csdf.Var]Promote{}
	first := map[csdf.Var]Promote{}
	seen := map[[2]string]Promote{}

	for _, p := range e.global.Promotes {
		if p.In == "" {
			e.errorf(p.Line, "the <<promote>> block of %q is not inside a state; write the state it moves in as a composite state around it", p.Map)
		} else if !holdsVar(e.global.Core.States[p.In], p.Map) {
			e.errorf(p.Line, "state %q holds no state variable named %q", p.In, p.Map)
		}

		if v, ok := varOf(e.global.Core.States[p.In], p.Map); ok && !strings.Contains(v.Type, p.Type) {
			e.warnf(p.Line, "%q is promoted as %q but its state variable is written %q", p.Map, p.Type, v.Type)
		}

		key := [2]string{string(p.In), string(p.Map)}
		if prev, ok := seen[key]; ok {
			e.errorf(p.Line, "%q is promoted twice in state %q; the first block is at line %d", p.Map, p.In, prev.Line)
		} else {
			seen[key] = p
		}

		if prev, ok := first[p.Map]; !ok {
			first[p.Map] = p
		} else if p.IDParam != prev.IDParam || p.Type != prev.Type {
			e.errorf(p.Line, "%q is written as %q here and as %q at line %d",
				p.Map,
				fmt.Sprintf("%s ⇸ %s", p.IDParam, p.Type),
				fmt.Sprintf("%s ⇸ %s", prev.IDParam, prev.Type),
				prev.Line)
		}

		if p.Path == "" {
			continue
		}
		if prev, ok := includers[p.Map]; ok {
			e.errorf(p.Line, "%q already includes its local diagram at line %d; a second !include of one file collides with the first on every state ID", p.Map, prev.Line)
			continue
		}
		includers[p.Map] = p
	}

	for m, p := range first {
		if _, ok := includers[m]; !ok {
			e.errorf(p.Line, "no <<promote>> block of %q carries an !include, so it has no local diagram", m)
		}
	}
}

// loadLocals reads the local diagram of every map that has exactly one !include.
// A map whose blocks are already in error is left alone: there is no one file to
// read for it.
func (e *expander) loadLocals() {
	for _, p := range e.global.Promotes {
		if p.Path == "" {
			continue
		}
		if _, done := e.locals[p.Map]; done {
			continue
		}

		local, err := e.load(p.Path)
		if err != nil {
			e.errorf(p.Line, "cannot read the local diagram %q: %v", p.Path, err)
			continue
		}
		e.locals[p.Map] = local
		e.paths[p.Map] = p.Path
		e.order = append(e.order, p)
	}
}

// checkLocals reads each local diagram against the contract a promoted diagram
// has to keep: the start state means that the instance does not exist.
func (e *expander) checkLocals() {
	declaredIn := map[csdf.StateID]string{}

	for _, p := range e.order {
		local := e.locals[p.Map]
		absent := local.StartEdge.Dst

		if len(local.States[absent].Vars) > 0 {
			e.errorf(p.Line, "the start state %q of %s holds state variables; it means that the instance does not exist", absent, p.Path)
		}
		if local.EndEdge != nil {
			e.errorf(p.Line, "%s has an end edge; write a state with no transition out of it instead", p.Path)
		}
		for _, edge := range local.Edges {
			if edge.Src == absent && edge.Dst == absent {
				e.errorf(p.Line, "%s has an edge from its start state %q back to itself, which is an event of an instance that does not exist", p.Path, absent)
				break
			}
		}
		for _, edge := range local.Edges {
			// A promoted tau carries no instance ID, so its instance is only
			// existentially quantified. On a creation edge that leaves the guard
			// saying "some ID is not in the map yet", which is true as long as
			// there are IDs left: instances would appear out of nothing.
			if edge.Src == absent && edge.Event == csdf.Tau {
				e.errorf(p.Line, "%s creates an instance on tau, whose instance is only existentially quantified, so nothing stops instances from appearing", p.Path)
				break
			}
		}

		for _, s := range csdf.SortedStates(local.States) {
			if other, ok := declaredIn[s.ID]; ok {
				e.errorf(p.Line, "%s and %s both declare the state %q; !include drops every local diagram into one namespace", p.Path, other, s.ID)
				continue
			}
			declaredIn[s.ID] = p.Path
		}
	}
}

func holdsVar(s csdf.State, name csdf.Var) bool {
	_, ok := varOf(s, name)
	return ok
}

func varOf(s csdf.State, name csdf.Var) (csdf.StateVar, bool) {
	i := slices.IndexFunc(s.Vars, func(v csdf.StateVar) bool { return v.Name == name })
	if i < 0 {
		return csdf.StateVar{}, false
	}
	return s.Vars[i], true
}

func (e *expander) errorf(line int, format string, args ...any) {
	e.diags = append(e.diags, Diagnostic{Severity: SeverityError, Line: line, Message: fmt.Sprintf(format, args...)})
}

// Refusal returns why a run of the expansion should fail, or nil when it should
// not. An error leaves the diagram unprinted; werror says whether a warning does
// too. The policy lives here rather than in the CLI so that any other front end
// refuses the same diagrams.
func Refusal(diags []Diagnostic, werror bool) error {
	if HasError(diags) {
		return errors.New("the promotion has errors")
	}
	if werror && HasSeverity(diags, SeverityWarning) {
		return errors.New("the promotion has warnings and -Werror is set")
	}
	return nil
}

// HasError reports whether any diagnostic leaves the diagram unprintable.
func HasError(diags []Diagnostic) bool { return HasSeverity(diags, SeverityError) }

// HasSeverity reports whether any diagnostic is of that severity. -Werror is a
// policy over this, not over the printing.
func HasSeverity(diags []Diagnostic, s Severity) bool {
	return slices.ContainsFunc(diags, func(d Diagnostic) bool { return d.Severity == s })
}

// sorted returns the diagnostics by source line, so that reading them follows
// reading the file. The checks run in an order of their own, and one of them
// ranges over a map. The ones about no line in particular go last, after
// everything a reader can point at.
func (e *expander) sorted() []Diagnostic {
	slices.SortStableFunc(e.diags, func(a, b Diagnostic) int {
		return lineOrder(a.Line) - lineOrder(b.Line)
	})
	return e.diags
}

func lineOrder(line int) int {
	if line == 0 {
		return math.MaxInt32
	}
	return line
}

// warn reports what is probably not meant. None of it stops the expansion: the
// diagram it produces is well formed, just unlikely to be what the author had
// in mind.
func (e *expander) warn() {
	e.warnAboutLocals()
	e.warnAboutSharedEvents()
	e.warnAboutNotes()
	e.warnAboutFrozenMaps()
}

// warnAboutLocals reports the posts the expansion throws away.
func (e *expander) warnAboutLocals() {
	for _, p := range e.order {
		local := e.locals[p.Map]
		absent := local.StartEdge.Dst

		if !csdf.IsTrue(local.StartEdge.Post) {
			e.warnf(p.Line, "the start edge of %s has a post, which says nothing: its state means that the instance does not exist. Leave it out", p.Path)
		}
		for _, edge := range local.Edges {
			if edge.Dst == absent && !csdf.IsTrue(edge.Post) {
				e.warnf(p.Line, "the post of %s in %s is dropped: the edge deletes the instance it would be about. Write an effect on another map as a sync or a hand-written edge", edge.Event, p.Path)
			}
		}
	}
}

// warnAboutSharedEvents reports an event two families have without a sync. Two
// families that name an event the same way usually mean to take it together;
// without a sync they take it one at a time.
func (e *expander) warnAboutSharedEvents() {
	synced := map[string]bool{}
	for _, s := range e.global.Syncs {
		synced[s.Event] = true
	}

	seen := map[string]Promote{}
	for _, p := range e.order {
		for _, event := range eventsOf(e.locals[p.Map]) {
			if synced[event] {
				continue
			}
			if prev, ok := seen[event]; ok && prev.Map != p.Map {
				e.warnf(0, "%s and %s both have an edge on %q, which is not synced; each family takes it on its own", prev.Path, p.Path, event)
				continue
			}
			seen[event] = p
		}
	}
}

// warnAboutNotes reports a directive whose note points somewhere its own maps do
// not live, and a constrain whose guard says nothing about its instance.
func (e *expander) warnAboutNotes() {
	for _, s := range e.global.Syncs {
		maps := make([]csdf.Var, 0, len(s.Targets))
		for _, t := range s.Targets {
			maps = append(maps, t.Map)
		}
		e.warnAboutAnchor(s.Anchor, s.Line, maps)
	}

	for _, c := range e.global.Constrains {
		e.warnAboutAnchor(c.Anchor, c.Line, nil)

		named := slices.ContainsFunc(c.Params, func(p string) bool {
			return strings.Contains(string(c.Guard), p)
		})
		if !named {
			e.warnf(c.Line, "the guard of this constrain names none of its arguments, so it says nothing about the instance")
		}
	}
}

// warnAboutAnchor reports a note pointing at a block of a map the directive does
// not name. A constrain names no map of its own, so any block it points at is
// one the reader is meant to look at; only a sync can be checked.
func (e *expander) warnAboutAnchor(a Anchor, line int, maps []csdf.Var) {
	if a.State == "" || len(maps) == 0 {
		return
	}
	for _, p := range e.global.Promotes {
		if p.Alias == a.State && slices.Contains(maps, p.Map) {
			return
		}
	}
	e.warnf(line, "this note points at %q, which is not a block of any map it names", a.State)
}

// warnAboutFrozenMaps reports a state that holds a map but has no block for it.
// It is a thing an author may well mean - a mode in which that family does not
// move - so it is only worth saying out loud.
func (e *expander) warnAboutFrozenMaps() {
	promoted := map[csdf.Var]bool{}
	for _, p := range e.global.Promotes {
		promoted[p.Map] = true
	}

	for _, state := range csdf.SortedStates(e.global.Core.States) {
		for _, v := range state.Vars {
			if !promoted[v.Name] {
				continue
			}
			if _, ok := e.blockOf(v.Name, state.ID); ok {
				continue
			}
			e.infof("state %q holds %q but has no <<promote>> block for it, so the family is frozen there", state.ID, v.Name)
		}
	}
}

// eventsOf returns the event names a local diagram uses, in source order.
func eventsOf(local *csdf.Diagram) []string {
	var events []string
	for _, edge := range local.Edges {
		if edge.Event == csdf.Tau {
			continue
		}
		if name, _ := splitEvent(edge.Event); !slices.Contains(events, name) {
			events = append(events, name)
		}
	}
	return events
}

func (e *expander) warnf(line int, format string, args ...any) {
	e.diags = append(e.diags, Diagnostic{Severity: SeverityWarning, Line: line, Message: fmt.Sprintf(format, args...)})
}

func (e *expander) infof(format string, args ...any) {
	e.diags = append(e.diags, Diagnostic{Severity: SeverityInfo, Message: fmt.Sprintf(format, args...)})
}
