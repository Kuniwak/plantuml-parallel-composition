package promote_test

import (
	"fmt"
	"strings"
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

func TestExpandFramesTheOtherMapsAndKeepsTauSilent(t *testing.T) {
	// Arrange: a global state with a second map, so that every promoted edge
	// has something to leave alone.
	global := csdf.MustParse(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
running : audits ; 監査ID ⇸ Audit
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`)
	load := stubLoader(map[string]string{
		"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
state "凍結中" as frozen
[*] --> none : 残高は 0
none --> open : OPEN
open --> frozen : FREEZE(理由) ; 残高が 0 でない ; 凍結理由は 理由
frozen --> open : tau
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
			Event: "FREEZE(口座ID, 理由)",
			Guard: "口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ∧ 残高が 0 でない",
			Post:  "accounts' = accounts ⊕ {口座ID ↦ 〈凍結中〉} ∧ 凍結理由は 理由 ∧ audits' = audits",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "OPEN(口座ID)",
			Guard: "口座ID ∉ dom accounts",
			Post:  "accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉} ∧ 残高は 0 ∧ audits' = audits",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "tau",
			Guard: "口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈凍結中〉",
			Post:  "accounts' = accounts ⊕ {口座ID ↦ 〈開設済み〉} ∧ audits' = audits",
		},
	}
	if diff := cmp.Diff(want, result.Diagram.Edges); diff != "" {
		t.Errorf("Expand() edges mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandCopiesThePromotionIntoEveryStateOfTheInClause(t *testing.T) {
	// Arrange: the same map is driven in two operating modes, and the edge that
	// switches between them is hand-written.
	global := csdf.MustParse(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
state "縮退中" as degraded
degraded : accounts ; 口座ID ⇸ Account
[*] --> running
running --> degraded : DEGRADE ; true ; accounts' = accounts
promote local/ACCOUNT.puml as Account via accounts(口座ID) in running, degraded
@enduml
`)
	load := stubLoader(map[string]string{
		"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
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
			Src:   "degraded",
			Dst:   "degraded",
			Event: "OPEN(口座ID)",
			Guard: "口座ID ∉ dom accounts",
			Post:  "accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}",
		},
		{
			Src:   "running",
			Dst:   "degraded",
			Event: "DEGRADE",
			Guard: "true",
			Post:  "accounts' = accounts",
			Line:  7,
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
}

// diagnosticsOf runs Expand and returns its diagnostics of one severity as
// strings, so that a test can say what it expects without naming line numbers
// twice.
func diagnosticsOf(t *testing.T, global *csdf.Diagram, load promote.Loader, severity promote.Severity) []string {
	t.Helper()
	_, diags := promote.Expand(global, load)
	var got []string
	for _, diag := range diags {
		if diag.Severity == severity {
			got = append(got, diag.Message)
		}
	}
	return got
}

func TestExpandReportsStructuralErrors(t *testing.T) {
	local := `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`

	type testCase struct {
		Global  string
		Sources map[string]string
		Want    []string
	}

	testCases := map[string]testCase{
		"the promoted map is not a state variable of the state": {
			Global: `@startuml
state "稼働中" as running
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`promote into "running": the state has no state variable "accounts" to promote through`},
		},
		"the in clause names a state that does not exist": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID) in maintenance
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`promote into "maintenance": no such state in this diagram`},
		},
		"the local diagram cannot be read": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/MISSING.puml as Account via accounts(口座ID)
@enduml
`,
			Sources: map[string]string{},
			Want:    []string{`promote local/MISSING.puml: no such file: "local/MISSING.puml"`},
		},
		"the start state of the local diagram holds state variables": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
none : balance
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`},
			Want: []string{`promote local/ACCOUNT.puml: the start state "none" means that no such instance exists, so it must have no state variables`},
		},
		"the local diagram has an end edge": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
open --> [*]
@enduml
`},
			Want: []string{`promote local/ACCOUNT.puml: an end edge cannot be promoted; write a state with no outgoing edge instead`},
		},
		"the same map is promoted twice": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
promote local/ACCOUNT.puml as Account via accounts(別の口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`promote via "accounts": the map is already promoted at line 5`},
		},
		"a synced map is not promoted": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
sync OPEN : accounts(口座ID), audits(監査ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`sync OPEN: the map "audits" is not promoted`},
		},
		"a synced event is missing from a local diagram": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
sync CLOSE : accounts(口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`sync CLOSE: local/ACCOUNT.puml has no such event`},
		},
		"the promotions of the synced maps share no state": {
			Global: `@startuml
state "稼働中" as running
running : accounts
running : audits
state "縮退中" as degraded
degraded : accounts
degraded : audits
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID) in running
promote local/ACCOUNT.puml as Audit via audits(監査ID) in degraded
sync OPEN : accounts(口座ID), audits(監査ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": local},
			Want:    []string{`sync OPEN: the promotions of its maps share no state, so the event can never happen`},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := diagnosticsOf(t, csdf.MustParse(tc.Global), stubLoader(tc.Sources), promote.SeverityError)
			if diff := cmp.Diff(tc.Want, got); diff != "" {
				t.Errorf("errors mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExpandMergesSyncedEdges(t *testing.T) {
	// Arrange: booking a buy trade and counting it into the segregation report
	// is one event, so the two instances must take it together.
	global := csdf.MustParse(`@startuml
state "稼働中" as running
running : buys ; 約定ID ⇸ Buy
running : cycles ; 基準日 ⇸ Cycle
[*] --> running
promote local/BUY.puml as Buy via buys(約定ID)
promote local/CYCLE.puml as Cycle via cycles(基準日)
sync BOOK : buys(約定ID), cycles(基準日)
@enduml
`)
	load := stubLoader(map[string]string{
		"local/BUY.puml": `@startuml
state "なし" as none
state "約定済み" as booked
[*] --> none
none --> booked : BOOK(数量)
@enduml
`,
		"local/CYCLE.puml": `@startuml
state "未開始" as idle
state "集計中" as counting
[*] --> idle
idle --> counting : BOOK(数量)
counting --> counting : BOOK(数量)
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
			Event: "BOOK(約定ID, 基準日, 数量)",
			Guard: "約定ID ∉ dom buys ∧ 基準日 ∈ dom cycles ∧ cycles(基準日) ∈ 〈集計中〉",
			Post:  "buys' = buys ∪ {約定ID ↦ 〈約定済み〉} ∧ cycles' = cycles ⊕ {基準日 ↦ 〈集計中〉}",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "BOOK(約定ID, 基準日, 数量)",
			Guard: "約定ID ∉ dom buys ∧ 基準日 ∉ dom cycles",
			Post:  "buys' = buys ∪ {約定ID ↦ 〈約定済み〉} ∧ cycles' = cycles ∪ {基準日 ↦ 〈集計中〉}",
		},
	}
	if diff := cmp.Diff(want, result.Diagram.Edges); diff != "" {
		t.Errorf("Expand() edges mismatch (-want +got):\n%s", diff)
	}
	for i, origin := range result.Origins {
		if !strings.HasPrefix(origin, "sync: BOOK ") {
			t.Errorf("Origins[%d] = %q; want it to record the sync", i, origin)
		}
	}
}
