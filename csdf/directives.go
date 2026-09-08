package csdf

// Promotion directives (docs/PROMOTION.md) turn a global diagram into a
// promotion of one or more local diagrams. They are consumed by csdfpromote and
// rejected by every other tool, so no diagram that reaches an analysis carries
// them.

// Promote promotes the local diagram at Path through the state variable Map,
// which holds a partial map from instance ids to local states.
type Promote struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Map     Var    `json:"map"`
	IDParam string `json:"id_param"`
	// In lists the global states the local diagram is expanded into. Empty
	// means the destination of the start edge alone.
	In   []StateID `json:"in"`
	Line int       `json:"line"` // 1-based source line of the directive.
}

// MapRef names a promoted map together with the parameter that stands for its
// instance id in the directive that mentions it.
type MapRef struct {
	Map   Var    `json:"map"`
	Param string `json:"param"`
}

// Sync merges the edges that the named local event contributes to each of the
// referenced maps into a single global edge, so that the instances take the
// event together instead of independently.
type Sync struct {
	// Event is the local event name: everything before the argument list.
	Event   string   `json:"event"`
	Targets []MapRef `json:"targets"`
	Line    int      `json:"line"` // 1-based source line of the directive.
}

// Constrain conjoins Guard onto the guard of every expanded edge whose event
// name and arity match. Event and Params are written in the promoted form, so
// the first parameter is the instance id.
type Constrain struct {
	Event  string    `json:"event"`
	Params []string  `json:"params"`
	Guard  Predicate `json:"guard"`
	Line   int       `json:"line"` // 1-based source line of the directive.
}

// HasDirectives reports whether the diagram still carries promotion directives.
// A tool other than csdfpromote must refuse such a diagram: its edges are not
// the whole of its behaviour.
func (d *Diagram) HasDirectives() bool {
	return len(d.Promotes) > 0 || len(d.Syncs) > 0 || len(d.Constrains) > 0
}
