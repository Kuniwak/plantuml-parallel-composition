package promote_test

import (
	"fmt"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// stubLoader answers with the diagrams it was built from, and reports every
// other path as missing, the way the file system would.
func stubLoader(sources map[string]string) promote.Loader {
	return func(path string) (*csdf.Diagram, error) {
		source, ok := sources[path]
		if !ok {
			return nil, fmt.Errorf("no such file: %q", path)
		}
		return csdf.Parse(source)
	}
}

func TestExpandPromotesEveryLocalEdge(t *testing.T) {
	// Arrange: the local diagram's start state means "no such instance", so
	// leaving it creates one and entering it deletes one.
	global := csdf.MustParse(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`)
	load := stubLoader(map[string]string{
		"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
open --> none : CLOSE
@enduml
`,
	})

	// Act
	result, diags := promote.Expand(global, load)

	// Assert
	if got := promote.Errors(diags); len(got) > 0 {
		t.Fatalf("Expand() reported errors: %v", got)
	}

	want := []csdf.Edge{
		{
			Src:   "running",
			Dst:   "running",
			Event: "CLOSE(口座ID)",
			Guard: "口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉",
			Post:  "accounts' = {口座ID} ⩤ accounts",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "OPEN(口座ID)",
			Guard: "口座ID ∉ dom accounts",
			Post:  "accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}",
		},
	}
	if diff := cmp.Diff(want, result.Diagram.Edges); diff != "" {
		t.Errorf("Expand() edges mismatch (-want +got):\n%s", diff)
	}
	if result.Diagram.HasDirectives() {
		t.Errorf("Expand() left directives on the diagram")
	}
}
