package csdf

import (
	"strings"
	"testing"
)

func TestParsePromoteDirective(t *testing.T) {
	input := `@startuml
state "running" as running
running : accounts ; ID ⇸ Account
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() = _, %v; want no error", err)
	}

	if len(diagram.Promotes) != 1 {
		t.Fatalf("len(Promotes) = %d; want 1", len(diagram.Promotes))
	}

	got := diagram.Promotes[0]
	want := Promote{
		Path:    "local/ACCOUNT.puml",
		Type:    "Account",
		Map:     "accounts",
		IDParam: "口座ID",
		In:      nil,
		Line:    5,
	}
	if got.Path != want.Path || got.Type != want.Type || got.Map != want.Map || got.IDParam != want.IDParam || got.Line != want.Line {
		t.Errorf("Promotes[0] = %+v; want %+v", got, want)
	}
	if len(got.In) != 0 {
		t.Errorf("Promotes[0].In = %v; want empty", got.In)
	}
}

func TestParsePromoteDirectiveWithInClause(t *testing.T) {
	input := `@startuml
state "running" as running
running : accounts ; ID ⇸ Account
state "degraded" as degraded
degraded : accounts ; ID ⇸ Account
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID) in running, degraded
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() = _, %v; want no error", err)
	}

	if len(diagram.Promotes) != 1 {
		t.Fatalf("len(Promotes) = %d; want 1", len(diagram.Promotes))
	}

	got := diagram.Promotes[0].In
	want := []StateID{"running", "degraded"}
	if len(got) != len(want) {
		t.Fatalf("Promotes[0].In = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Promotes[0].In[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestParseSyncDirective(t *testing.T) {
	input := `@startuml
state "running" as running
running : buyTrades
running : segReports
[*] --> running
sync EVT-BOOK : buyTrades(買い約定ID), segReports(基準日)
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() = _, %v; want no error", err)
	}

	if len(diagram.Syncs) != 1 {
		t.Fatalf("len(Syncs) = %d; want 1", len(diagram.Syncs))
	}

	got := diagram.Syncs[0]
	if got.Event != "EVT-BOOK" {
		t.Errorf("Syncs[0].Event = %q; want %q", got.Event, "EVT-BOOK")
	}
	if got.Line != 6 {
		t.Errorf("Syncs[0].Line = %d; want 6", got.Line)
	}
	want := []MapRef{
		{Map: "buyTrades", Param: "買い約定ID"},
		{Map: "segReports", Param: "基準日"},
	}
	if len(got.Targets) != len(want) {
		t.Fatalf("Syncs[0].Targets = %v; want %v", got.Targets, want)
	}
	for i := range want {
		if got.Targets[i] != want[i] {
			t.Errorf("Syncs[0].Targets[%d] = %+v; want %+v", i, got.Targets[i], want[i])
		}
	}
}

func TestParseConstrainDirective(t *testing.T) {
	input := `@startuml
state "running" as running
running : sessions
[*] --> running
constrain EVT-APPROVE(エントリID, checker) ; sessions の checker が〈ログイン中〉
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() = _, %v; want no error", err)
	}

	if len(diagram.Constrains) != 1 {
		t.Fatalf("len(Constrains) = %d; want 1", len(diagram.Constrains))
	}

	got := diagram.Constrains[0]
	if got.Event != "EVT-APPROVE" {
		t.Errorf("Constrains[0].Event = %q; want %q", got.Event, "EVT-APPROVE")
	}
	if got.Guard != "sessions の checker が〈ログイン中〉" {
		t.Errorf("Constrains[0].Guard = %q; want %q", got.Guard, "sessions の checker が〈ログイン中〉")
	}
	if got.Line != 5 {
		t.Errorf("Constrains[0].Line = %d; want 5", got.Line)
	}
	want := []string{"エントリID", "checker"}
	if len(got.Params) != len(want) {
		t.Fatalf("Constrains[0].Params = %v; want %v", got.Params, want)
	}
	for i := range want {
		if got.Params[i] != want[i] {
			t.Errorf("Constrains[0].Params[%d] = %q; want %q", i, got.Params[i], want[i])
		}
	}
}

func TestParseDirectiveKeywordAsStateID(t *testing.T) {
	// A state may be called "sync" or "promote": an edge is an edge, and only a
	// line that is not one can be a directive.
	input := `@startuml
state "Syncing" as sync
state "Promoting" as promote
[*] --> sync
sync --> promote : go
promote --> sync : back
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() = _, %v; want no error", err)
	}
	if len(diagram.Edges) != 2 {
		t.Fatalf("len(Edges) = %d; want 2", len(diagram.Edges))
	}
	if diagram.HasDirectives() {
		t.Errorf("HasDirectives() = true; want false")
	}
}

func TestParseRejectsMalformedDirectives(t *testing.T) {
	cases := map[string]string{
		"promote without via":          `promote local/A.puml as Account accounts(id)`,
		"promote without parentheses":  `promote local/A.puml as Account via accounts`,
		"promote without closing":      `promote local/A.puml as Account via accounts(id`,
		"promote with empty parameter": `promote local/A.puml as Account via accounts()`,
		"promote with empty in clause": `promote local/A.puml as Account via accounts(id) in`,
		"sync without colon":           `sync EVT-BOOK buyTrades(id)`,
		"sync without targets":         `sync EVT-BOOK :`,
		"constrain without guard":      `constrain EVT-APPROVE(id) ;`,
		"constrain without semicolon":  `constrain EVT-APPROVE(id)`,
		"constrain without parameters": `constrain EVT-APPROVE() ; anything`,
	}

	for name, directive := range cases {
		t.Run(name, func(t *testing.T) {
			input := "@startuml\nstate \"running\" as running\n[*] --> running\n" + directive + "\n@enduml\n"

			if _, err := NewParser(input).Parse(); err == nil {
				t.Errorf("Parse() = _, nil; want an error for %q", directive)
			}
		})
	}
}

func TestParseBytesRejectsDirectives(t *testing.T) {
	input := []byte(`@startuml
state "running" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`)

	_, err := ParseBytes(input)
	if err == nil {
		t.Fatalf("ParseBytes() = _, nil; want an error")
	}
	if !strings.Contains(err.Error(), "csdfpromote") {
		t.Errorf("ParseBytes() error = %q; want it to name csdfpromote", err)
	}

	if _, err := ParseBytesAllowingDirectives(input); err != nil {
		t.Errorf("ParseBytesAllowingDirectives() = _, %v; want no error", err)
	}
}
