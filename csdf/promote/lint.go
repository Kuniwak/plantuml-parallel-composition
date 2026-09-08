package promote

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// check reads the promotions for everything that would make the expansion
// unsound, and loads the local diagram of every map along the way. An expansion
// runs only when nothing here is an error.
func (e *expander) check() {
	e.checkBlocks()
	e.loadLocals()
	e.checkLocals()
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
	return slices.ContainsFunc(s.Vars, func(v csdf.StateVar) bool { return v.Name == name })
}

func (e *expander) errorf(line int, format string, args ...any) {
	e.diags = append(e.diags, Diagnostic{Severity: SeverityError, Line: line, Message: fmt.Sprintf(format, args...)})
}

// hasError reports whether any diagnostic leaves the diagram unprintable.
func hasError(diags []Diagnostic) bool {
	return slices.ContainsFunc(diags, func(d Diagnostic) bool { return d.Severity == SeverityError })
}
