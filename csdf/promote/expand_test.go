package promote_test

import (
	"fmt"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// loaderOf resolves !include paths from an in-memory table, so an expansion test
// says what the local diagram is next to what it expands into.
func loaderOf(locals map[string]string) promote.LoadFunc {
	return func(path string) (*csdf.Diagram, error) {
		source, ok := locals[path]
		if !ok {
			return nil, fmt.Errorf("no such file: %q", path)
		}
		return csdf.Parse(source)
	}
}

func TestExpandOneMapWithNoStateVariables(t *testing.T) {
	g, err := promote.ParseGlobal(`@startuml ACCOUNTS
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空
@enduml
`)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	load := loaderOf(map[string]string{
		"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
accOpen --> accNone : CLOSE
@enduml
`,
	})

	got, diags, err := promote.Expand(g, load)
	if err != nil {
		t.Fatalf("promote.Expand() error = %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("promote.Expand() diagnostics = %v, want none", diags)
	}

	want := `@startuml ACCOUNTS
state "稼働中" as running
running: accounts ; 口座ID ⇸ Account
[*] --> running : accounts は空
' promote: local/ACCOUNT.puml 〈開設済み〉 → 〈未開設〉
running --> running : CLOSE(口座ID) ; 口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ; accounts' = {口座ID} ⩤ accounts
' promote: local/ACCOUNT.puml 〈未開設〉 → 〈開設済み〉
running --> running : OPEN(口座ID) ; 口座ID ∉ dom accounts ; accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}
@enduml
`
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("promote.Expand() mismatch (-want +got):\n%s", diff)
	}
}
