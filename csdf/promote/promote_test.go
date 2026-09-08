package promote_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

func TestParseGlobalReadsOnePromoteBlock(t *testing.T) {
	source := `@startuml ACCOUNTS
!define PROMOTED

state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空
@enduml
`

	g, err := promote.ParseGlobal(source)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	want := []promote.Promote{{
		Map:     "accounts",
		IDParam: "口座ID",
		Type:    "Account",
		Path:    "local/ACCOUNT.puml",
		Alias:   "runningAccounts",
		In:      "running",
		Line:    7,
	}}
	if diff := cmp.Diff(want, g.Promotes); diff != "" {
		t.Errorf("promote.ParseGlobal() promotes mismatch (-want +got):\n%s", diff)
	}
}

func TestParseGlobalFlattensTheCompositeState(t *testing.T) {
	source := `@startuml ACCOUNTS
!define PROMOTED

state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空
@enduml
`

	g, err := promote.ParseGlobal(source)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	want := &csdf.Diagram{
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
	}
	if diff := cmp.Diff(want, g.Core); diff != "" {
		t.Errorf("promote.ParseGlobal() core mismatch (-want +got):\n%s", diff)
	}
}

func TestParseGlobalReadsTheNoteDirectives(t *testing.T) {
	source := `@startuml TRADES
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
`

	g, err := promote.ParseGlobal(source)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	wantSyncs := []promote.Sync{{
		Anchor: promote.Anchor{NoteID: "sync1"},
		Event:  "BOOK",
		Targets: []promote.MapRef{
			{Map: "buys", Param: "約定ID"},
			{Map: "cycles", Param: "基準日"},
		},
		Line: 17,
	}}
	if diff := cmp.Diff(wantSyncs, g.Syncs); diff != "" {
		t.Errorf("promote.ParseGlobal() syncs mismatch (-want +got):\n%s", diff)
	}

	wantConstrains := []promote.Constrain{{
		Anchor: promote.Anchor{State: "runningBuys"},
		Event:  "BUY",
		Params: []string{"約定ID", "数量"},
		Guard:  "数量 は最小取引単位の倍数である",
		Line:   21,
	}}
	if diff := cmp.Diff(wantConstrains, g.Constrains); diff != "" {
		t.Errorf("promote.ParseGlobal() constrains mismatch (-want +got):\n%s", diff)
	}
}
