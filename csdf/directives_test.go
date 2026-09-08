package csdf

import "testing"

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
