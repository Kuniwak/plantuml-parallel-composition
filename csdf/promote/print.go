package promote

// String prints the expanded diagram as PlantUML, with a line comment above each
// generated edge saying which local edge it came from.
func (x *Expansion) String() string {
	return x.Diagram.StringWithEdgeComments(func(i int) string { return x.Edges[i].Origin })
}

// StringWithoutComments is String with the origin comments left out.
func (x *Expansion) StringWithoutComments() string { return x.Diagram.String() }
