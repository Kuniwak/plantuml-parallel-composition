// Package promote reads the upper-compatible grammar in which a global diagram
// declares a promotion, and expands it into a plain Composable State Diagram.
//
// The directives are spelled in PlantUML's own syntax - a composite state per
// state, a <<promote>> block per map, a note per sync or constrain - so that the
// diagram an author writes is also the picture a reader looks at. The core csdf
// grammar is untouched: what this package does is lift the directives out and
// hand the rest to csdf.Parse.
package promote

import "github.com/Kuniwak/puml-parallel/csdf"

// GlobalDiagram is a global diagram together with the directives lifted out of
// it. Core is what is left once they are gone: the composite states flattened
// into ordinary ones, and nothing else changed.
type GlobalDiagram struct {
	Core       *csdf.Diagram `json:"core"`
	Promotes   []Promote     `json:"promotes"`
	Syncs      []Sync        `json:"syncs"`
	Constrains []Constrain   `json:"constrains"`
	// Diagnostics is what reading the source had to say, which is nothing that
	// stops the parse: a stray !include, a note whose first word looks like a
	// directive misspelled.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Promote is one <<promote>> block: the family of instances that the map holds,
// living in the state the block was written in.
//
// Path is empty for a block with no body, which only says that the family moves
// in that state too. The local diagram then comes from the map's other block -
// the one that does carry the !include. Two blocks cannot both include the file,
// because PlantUML would merge their states on the shared IDs.
type Promote struct {
	Map     csdf.Var     `json:"map"`
	IDParam string       `json:"id_param"`
	Type    string       `json:"type"`
	Path    string       `json:"path,omitempty"`
	Alias   csdf.StateID `json:"alias"`
	In      csdf.StateID `json:"in"`
	Line    int          `json:"line"` // 1-based source line of the block.
}

// Sync is one sync directive: the maps that take the event together.
type Sync struct {
	Anchor  Anchor   `json:"anchor"`
	Event   string   `json:"event"`
	Targets []MapRef `json:"targets"`
	Line    int      `json:"line"` // 1-based source line of the directive.
}

// MapRef names a map and the parameter the instance ID is written as.
type MapRef struct {
	Map   csdf.Var `json:"map"`
	Param string   `json:"param"`
}

// Constrain is one constrain directive: a guard conjoined onto every expanded
// edge that matches the event in name and arity.
type Constrain struct {
	Anchor Anchor         `json:"anchor"`
	Event  string         `json:"event"`
	Params []string       `json:"params"`
	Guard  csdf.Predicate `json:"guard"`
	Line   int            `json:"line"` // 1-based source line of the directive.
}

// Anchor is how a note was written. A floating "note as <id>" leaves State
// empty; "note <dir> of <state>" records the state it points at, which is what
// the lint checks the directive's own maps against.
type Anchor struct {
	NoteID string       `json:"note_id,omitempty"`
	State  csdf.StateID `json:"state,omitempty"`
}
