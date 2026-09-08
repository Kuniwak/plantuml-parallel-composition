package promote_test

import (
	"testing"

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
