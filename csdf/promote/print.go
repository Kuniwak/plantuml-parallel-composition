package promote

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// String prints the expanded diagram as PlantUML, with a line comment above each
// generated edge saying which local edge it came from.
//
// This repeats what csdf.Diagram.String does, because the comments have to sit
// between the edges and the core printer has nowhere to put them. Keep the two
// in step: the point of the origin comments is a diagram that still reads like
// every other tool's output.
func (x *Expansion) String() string { return x.print(true) }

// StringWithoutComments is String with the origin comments left out.
func (x *Expansion) StringWithoutComments() string { return x.print(false) }

func (x *Expansion) print(comments bool) string {
	d := x.Diagram

	var sb strings.Builder
	if d.Name == "" {
		sb.WriteString("@startuml\n")
	} else {
		sb.WriteString(fmt.Sprintf("@startuml %s\n", d.Name))
	}

	for _, state := range csdf.SortedStates(d.States) {
		sb.WriteString(fmt.Sprintf("state %q as %s\n", state.Name, state.ID))
		for _, v := range state.Vars {
			sb.WriteString(fmt.Sprintf("%s: %s", state.ID, v.Name))
			if v.Type != "" {
				sb.WriteString(fmt.Sprintf(" ; %s", v.Type))
			}
			sb.WriteString("\n")
		}
	}

	if csdf.IsTrue(d.StartEdge.Post) {
		sb.WriteString(fmt.Sprintf("[*] --> %s\n", d.StartEdge.Dst))
	} else {
		sb.WriteString(fmt.Sprintf("[*] --> %s : %s\n", d.StartEdge.Dst, d.StartEdge.Post))
	}

	for i, edge := range d.Edges {
		if comments && x.Origins[i] != "" {
			sb.WriteString(fmt.Sprintf("' %s\n", x.Origins[i]))
		}
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", edge.Src, edge.Dst, edge.Event))
		// A lone "; x" is a guard (docs/SYNTAX.md), so an edge that only has a
		// post must spell the omitted guard out as "; true ; x".
		if csdf.IsTrue(edge.Post) {
			if !csdf.IsTrue(edge.Guard) {
				sb.WriteString(fmt.Sprintf(" ; %s", edge.Guard))
			}
			sb.WriteString("\n")
			continue
		}
		guard := edge.Guard
		if csdf.IsTrue(guard) {
			guard = csdf.PredicateTrue
		}
		sb.WriteString(fmt.Sprintf(" ; %s ; %s\n", guard, edge.Post))
	}

	if d.EndEdge != nil {
		sb.WriteString(fmt.Sprintf("%s --> [*]", d.EndEdge.Src))
		if !csdf.IsTrue(d.EndEdge.Guard) {
			sb.WriteString(fmt.Sprintf(" : %s", d.EndEdge.Guard))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
