package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// The four shapes a directive is written in, read out of one diagram each: what
// ParseGlobal lifts, and what it leaves for csdf.Parse.
func TestParseGlobal(t *testing.T) {
	cases := map[string]struct {
		source         string
		wantCore       *csdf.Diagram
		wantPromotes   []promote.Promote
		wantSyncs      []promote.Sync
		wantConstrains []promote.Constrain
	}{
		"a promote block, and the state it flattens into": {
			source: `@startuml ACCOUNTS
!define PROMOTED

state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空
@enduml
`,
			wantCore: &csdf.Diagram{
				Name: "ACCOUNTS",
				States: map[csdf.StateID]csdf.State{
					"running": {
						Name: "稼働中",
						Vars: []csdf.StateVar{{Name: "accounts", Type: "口座ID ⇸ Account"}},
						Line: 4,
					},
				},
				StartEdge: csdf.StartEdge{Dst: "running", Post: "accounts は空", Line: 12},
				Edges:     []csdf.Edge{},
			},
			wantPromotes: []promote.Promote{{
				Map: "accounts", IDParam: "口座ID", Type: "Account",
				Path: "local/ACCOUNT.puml", Alias: "runningAccounts", In: "running", Line: 7,
			}},
		},

		"a sync note, a constrain note and a note for the reader": {
			source: `@startuml TRADES
state "稼働中" as running {
  running : buys ; 約定ID ⇸ Buy
  running : cycles ; 基準日 ⇸ Cycle

  state "buys : 約定ID ⇸ Buy" as runningBuys <<promote>> {
    !include local/BUY.puml
  }
  state "cycles : 基準日 ⇸ Cycle" as runningCycles <<promote>> {
    !include local/CYCLE.puml
  }
}

[*] --> running : buys と cycles は空

note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)
end note

note bottom of runningBuys
  constrain BUY(約定ID, 数量) ; 数量 は最小取引単位の倍数である
end note

note as drawing
  この注記は描画のためだけにある
end note
@enduml
`,
			wantPromotes: []promote.Promote{
				{
					Map: "buys", IDParam: "約定ID", Type: "Buy",
					Path: "local/BUY.puml", Alias: "runningBuys", In: "running", Line: 6,
				},
				{
					Map: "cycles", IDParam: "基準日", Type: "Cycle",
					Path: "local/CYCLE.puml", Alias: "runningCycles", In: "running", Line: 9,
				},
			},
			wantSyncs: []promote.Sync{{
				Anchor: promote.Anchor{NoteID: "sync1"},
				Event:  "BOOK",
				Targets: []promote.MapRef{
					{Map: "buys", Param: "約定ID"},
					{Map: "cycles", Param: "基準日"},
				},
				Line: 17,
			}},
			wantConstrains: []promote.Constrain{{
				Anchor: promote.Anchor{State: "runningBuys"},
				Event:  "BUY",
				Params: []string{"約定ID", "数量"},
				Guard:  "数量 は最小取引単位の倍数である",
				Line:   21,
			}},
		},

		// PlantUML lets a note sit inside the composite state it points at, and
		// that is where an author naturally writes the constrain of one family.
		"a note inside the composite state it points at": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }

  note bottom of runningAccounts
    constrain CLOSE(口座ID) ; 口座ID に未決済がない
  end note
}
[*] --> running
@enduml
`,
			wantPromotes: []promote.Promote{{
				Map: "accounts", IDParam: "口座ID", Type: "Account",
				Path: "local/ACCOUNT.puml", Alias: "runningAccounts", In: "running", Line: 5,
			}},
			wantConstrains: []promote.Constrain{{
				Anchor: promote.Anchor{State: "runningAccounts"},
				Event:  "CLOSE",
				Params: []string{"口座ID"},
				Guard:  "口座ID に未決済がない",
				Line:   10,
			}},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			g, err := promote.ParseGlobal(c.source)
			if err != nil {
				t.Fatalf("promote.ParseGlobal() error = %v", err)
			}

			if c.wantCore != nil {
				if diff := cmp.Diff(c.wantCore, g.Core); diff != "" {
					t.Errorf("promote.ParseGlobal() core mismatch (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(c.wantPromotes, g.Promotes); diff != "" {
				t.Errorf("promote.ParseGlobal() promotes mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantSyncs, g.Syncs); diff != "" {
				t.Errorf("promote.ParseGlobal() syncs mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantConstrains, g.Constrains); diff != "" {
				t.Errorf("promote.ParseGlobal() constrains mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseGlobalRefusals(t *testing.T) {
	cases := map[string]struct {
		source string
		want   string
	}{
		"a title that does not name a map": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			want: `line 4: expected a <<promote>> title of the form "<map> : <ID> ⇸ <Type>", got "accounts"`,
		},
		"a block body that is not an include": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    state "x" as x
  }
}
[*] --> running
@enduml
`,
			want: `line 5: expected a single !include in the <<promote>> block opened at line 4, got "state \"x\" as x"`,
		},
		"a block body with more than an include": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
    !include local/AUDIT.puml
  }
}
[*] --> running
@enduml
`,
			want: `line 4: expected a single !include in the <<promote>> block opened at line 4`,
		},
		"a composite state nested in another": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
  state "内側" as inner {
  }
}
[*] --> running
@enduml
`,
			want: `line 4: composite state is nested inside "running"; only a <<promote>> block may be nested`,
		},
		"a sync body that does not parse": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
}
[*] --> running
note as n1
  sync BOOK buys(約定ID)
end note
@enduml
`,
			want: `line 7: expected "sync <event> : <map>(<param>), ...", got "sync BOOK buys(約定ID)"`,
		},
		"a constrain body that does not parse": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
}
[*] --> running
note as n1
  constrain BUY(約定ID)
end note
@enduml
`,
			want: `line 7: expected "constrain <event>(<param>, ...) ; <guard>", got "constrain BUY(約定ID)"`,
		},
		"an unterminated note": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
}
[*] --> running
note as n1
  sync BOOK : buys(約定ID)
@enduml
`,
			want: `line 6: unterminated note`,
		},
		"an unterminated composite state": {
			source: `@startuml A
state "稼働中" as running {
  running : accounts
[*] --> running
@enduml
`,
			want: `unterminated composite state "running"`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := promote.ParseGlobal(c.source)
			if err == nil {
				t.Fatalf("promote.ParseGlobal() error = nil, want %q", c.want)
			}
			if got := err.Error(); !strings.Contains(got, c.want) {
				t.Errorf("promote.ParseGlobal() error = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

func TestLoadGlobalReadsTheWorkedExamples(t *testing.T) {
	cases := map[string]struct {
		path string
		want []promote.Promote
	}{
		"one map in one mode": {
			path: "../../examples/promote/ACCOUNTS.puml",
			want: []promote.Promote{{
				Map: "accounts", IDParam: "口座ID", Type: "Account",
				Path: "local/ACCOUNT.puml", Alias: "runningAccounts", In: "running", Line: 8,
			}},
		},
		"two maps and two modes": {
			path: "../../examples/promote/MODES.puml",
			want: []promote.Promote{
				{
					Map: "accounts", IDParam: "口座ID", Type: "Account",
					Path: "local/ACCOUNT.puml", Alias: "runningAccounts", In: "running", Line: 18,
				},
				{
					Map: "audits", IDParam: "監査ID", Type: "Audit",
					Path: "local/AUDIT.puml", Alias: "runningAudits", In: "running", Line: 21,
				},
				{
					Map: "accounts", IDParam: "口座ID", Type: "Account",
					Alias: "degradedAccounts", In: "degraded", Line: 30,
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			g, err := promote.LoadGlobal(c.path)
			if err != nil {
				t.Fatalf("promote.LoadGlobal(%q) error = %v", c.path, err)
			}
			if diff := cmp.Diff(c.want, g.Promotes); diff != "" {
				t.Errorf("promote.LoadGlobal(%q) promotes mismatch (-want +got):\n%s", c.path, diff)
			}
		})
	}
}

// PlantUML lets a note sit inside the composite state it points at, and that is
// where an author naturally writes the constrain of one family.
func TestParseGlobalReadsANoteInsideACompositeState(t *testing.T) {
	g, err := promote.ParseGlobal(`@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }

  note bottom of runningAccounts
    constrain CLOSE(口座ID) ; 口座ID に未決済がない
  end note
}
[*] --> running
@enduml
`)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	want := []promote.Constrain{{
		Anchor: promote.Anchor{State: "runningAccounts"},
		Event:  "CLOSE",
		Params: []string{"口座ID"},
		Guard:  "口座ID に未決済がない",
		Line:   10,
	}}
	if diff := cmp.Diff(want, g.Constrains); diff != "" {
		t.Errorf("promote.ParseGlobal() constrains mismatch (-want +got):\n%s", diff)
	}
}
