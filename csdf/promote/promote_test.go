package promote_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// mustParseGlobal parses a global diagram, promotion directives and all. The
// ordinary entry points refuse those, since csdfpromote is the only reader of a
// diagram that still has them.
func mustParseGlobal(source string) *csdf.Diagram {
	diagram, err := csdf.ParseAllowingDirectives(source)
	if err != nil {
		panic(err)
	}
	return diagram
}

// stubLoader answers with the diagrams it was built from, and reports every
// other path as missing, the way the file system would.
func stubLoader(sources map[string]string) csdf.DiagramLoader {
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
	global := mustParseGlobal(`@startuml
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
	result, diags := promote.Expand(global, load, promote.Options{})

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
	global := mustParseGlobal(`@startuml
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
	result, diags := promote.Expand(global, load, promote.Options{})

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
	global := mustParseGlobal(`@startuml
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
	result, diags := promote.Expand(global, load, promote.Options{})

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
func diagnosticsOf(t *testing.T, global *csdf.Diagram, load csdf.DiagramLoader, severity promote.Severity) []string {
	t.Helper()
	_, diags := promote.Expand(global, load, promote.Options{})
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
			got := diagnosticsOf(t, mustParseGlobal(tc.Global), stubLoader(tc.Sources), promote.SeverityError)
			if diff := cmp.Diff(tc.Want, got); diff != "" {
				t.Errorf("errors mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExpandMergesSyncedEdges(t *testing.T) {
	// Arrange: booking a buy trade and counting it into the segregation report
	// is one event, so the two instances must take it together.
	global := mustParseGlobal(`@startuml
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
	result, diags := promote.Expand(global, load, promote.Options{})

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

func TestExpandConjoinsConstraints(t *testing.T) {
	// Arrange: a checker may only approve while logged in, which is a fact
	// about another map and so cannot be written in the local diagram.
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
running : sessions ; 利用者 ⇸ Session
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
constrain OPEN(口座ID) ; 申込書が受理されている
constrain FREEZE(口座ID, 理由) ; 理由 は凍結事由の一覧にある
@enduml
`)
	load := stubLoader(map[string]string{
		"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
state "凍結中" as frozen
[*] --> none
none --> open : OPEN
open --> frozen : FREEZE(理由)
@enduml
`,
	})

	// Act
	result, diags := promote.Expand(global, load, promote.Options{})

	// Assert
	if got := promote.Errors(diags); len(got) > 0 {
		t.Fatalf("Expand() reported errors: %v", got)
	}

	want := []csdf.Edge{
		{
			Src:   "running",
			Dst:   "running",
			Event: "FREEZE(口座ID, 理由)",
			Guard: "口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ∧ 理由 は凍結事由の一覧にある",
			Post:  "accounts' = accounts ⊕ {口座ID ↦ 〈凍結中〉} ∧ sessions' = sessions",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "OPEN(口座ID)",
			Guard: "口座ID ∉ dom accounts ∧ 申込書が受理されている",
			Post:  "accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉} ∧ sessions' = sessions",
		},
	}
	if diff := cmp.Diff(want, result.Diagram.Edges); diff != "" {
		t.Errorf("Expand() edges mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandReportsAConstraintThatMatchesNothing(t *testing.T) {
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
constrain OPEN(口座ID, 理由) ; なにか
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

	want := []string{"constrain OPEN/2: no expanded edge carries that event with that many arguments"}
	if diff := cmp.Diff(want, diagnosticsOf(t, global, load, promote.SeverityError)); diff != "" {
		t.Errorf("errors mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandReportsWarnings(t *testing.T) {
	type testCase struct {
		Global  string
		Sources map[string]string
		Want    []string
	}

	testCases := map[string]testCase{
		"a shared event is not synced": {
			Global: `@startuml
state "稼働中" as running
running : accounts
running : audits
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
promote local/AUDIT.puml as Audit via audits(監査ID)
@enduml
`,
			Sources: map[string]string{
				"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`,
				"local/AUDIT.puml": `@startuml
state "なし" as none
state "記録済み" as done
[*] --> none
none --> done : OPEN
@enduml
`,
			},
			Want: []string{`the event OPEN is in local/ACCOUNT.puml and local/AUDIT.puml but is not synced; the instances take it independently`},
		},
		"the promoted type is not the type of the map": {
			Global: `@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ 口座
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`},
			Want: []string{`promote as Account: the type of "accounts" is "口座ID ⇸ 口座", which does not mention Account`},
		},
		"a deletion discards the local post": {
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
open --> none : CLOSE ; true ; 解約日は今日
@enduml
`},
			Want: []string{`promote local/ACCOUNT.puml: the edge 〈開設済み〉 → 〈未開設〉 deletes the instance, so its post "解約日は今日" is discarded`},
		},
		"a constraint mentions none of its parameters": {
			Global: `@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
constrain OPEN(口座ID) ; 受付時間内である
@enduml
`,
			Sources: map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`},
			Want: []string{`constrain OPEN/1: the guard mentions none of its parameters (口座ID)`},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			global := mustParseGlobal(tc.Global)
			load := stubLoader(tc.Sources)
			if got := diagnosticsOf(t, global, load, promote.SeverityError); len(got) > 0 {
				t.Fatalf("Expand() reported errors: %v", got)
			}
			if diff := cmp.Diff(tc.Want, diagnosticsOf(t, global, load, promote.SeverityWarning)); diff != "" {
				t.Errorf("warnings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExpandReadsBackAFrozenMap(t *testing.T) {
	// Arrange: the map is a state variable of both states, but the promotion
	// only drives it in one, so it is frozen in the other.
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
state "保守中" as maintenance
maintenance : accounts ; 口座ID ⇸ Account
[*] --> running
running --> maintenance : ENTER ; true ; accounts' = accounts
promote local/ACCOUNT.puml as Account via accounts(口座ID) in running
@enduml
`)
	load := stubLoader(map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
@enduml
`})

	want := []string{`promote via "accounts": the state "maintenance" holds the map but is not in the in clause, so the map is frozen there`}
	if diff := cmp.Diff(want, diagnosticsOf(t, global, load, promote.SeverityInfo)); diff != "" {
		t.Errorf("info mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandWithTemplates(t *testing.T) {
	// Arrange: the generated phrases are written in the language of the rest of
	// the specification, so they can be replaced wholesale.
	templates, err := promote.ParseTemplates(`
{{define "exists"}}{{.ID}} は {{.Map}} にある{{end}}
{{define "absent"}}{{.ID}} は {{.Map}} にない{{end}}
{{define "at"}}{{.Map}} の {{.ID}} は〈{{.Src}}〉である{{end}}
{{define "insert"}}{{.Map}} に {{.ID}} を〈{{.Dst}}〉として加える{{end}}
{{define "update"}}{{.Map}} の {{.ID}} を〈{{.Dst}}〉にする{{end}}
{{define "delete"}}{{.Map}} から {{.ID}} を除く{{end}}
{{define "unchanged"}}{{.Other}} は変わらない{{end}}
`)
	if err != nil {
		t.Fatalf("ParseTemplates() = _, %v; want no error", err)
	}

	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
running : audits ; 監査ID ⇸ Audit
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`)
	load := stubLoader(map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
state "凍結中" as frozen
[*] --> none
none --> open : OPEN
open --> frozen : FREEZE
frozen --> none : CLOSE
@enduml
`})

	// Act
	result, diags := promote.Expand(global, load, promote.Options{Templates: templates})

	// Assert
	if got := promote.Errors(diags); len(got) > 0 {
		t.Fatalf("Expand() reported errors: %v", got)
	}

	want := []csdf.Edge{
		{
			Src:   "running",
			Dst:   "running",
			Event: "CLOSE(口座ID)",
			Guard: "口座ID は accounts にある ∧ accounts の 口座ID は〈凍結中〉である",
			Post:  "accounts から 口座ID を除く ∧ audits は変わらない",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "FREEZE(口座ID)",
			Guard: "口座ID は accounts にある ∧ accounts の 口座ID は〈開設済み〉である",
			Post:  "accounts の 口座ID を〈凍結中〉にする ∧ audits は変わらない",
		},
		{
			Src:   "running",
			Dst:   "running",
			Event: "OPEN(口座ID)",
			Guard: "口座ID は accounts にない",
			Post:  "accounts に 口座ID を〈開設済み〉として加える ∧ audits は変わらない",
		},
	}
	if diff := cmp.Diff(want, result.Diagram.Edges); diff != "" {
		t.Errorf("Expand() edges mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandSyncTakesTheCartesianProduct(t *testing.T) {
	// Arrange: each side may contribute several edges for the same event, and
	// a guard on one side can pick out any state on the other, so every
	// combination is a possible behaviour.
	two := `@startuml
state "なし" as none
state "一つ" as one
state "二つ" as many
[*] --> none
none --> one : E
one --> many : E
@enduml
`
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : xs
running : ys
[*] --> running
promote local/X.puml as X via xs(x)
promote local/Y.puml as Y via ys(y)
sync E : xs(x), ys(y)
@enduml
`)
	load := stubLoader(map[string]string{"local/X.puml": two, "local/Y.puml": two})

	// Act
	result, diags := promote.Expand(global, load, promote.Options{})

	// Assert
	if got := promote.Errors(diags); len(got) > 0 {
		t.Fatalf("Expand() reported errors: %v", got)
	}
	if len(result.Diagram.Edges) != 4 {
		t.Fatalf("len(Edges) = %d; want 4 (2 x 2)", len(result.Diagram.Edges))
	}

	want := []csdf.Predicate{
		"x ∈ dom xs ∧ xs(x) ∈ 〈一つ〉 ∧ y ∈ dom ys ∧ ys(y) ∈ 〈一つ〉",
		"x ∈ dom xs ∧ xs(x) ∈ 〈一つ〉 ∧ y ∉ dom ys",
		"x ∉ dom xs ∧ y ∈ dom ys ∧ ys(y) ∈ 〈一つ〉",
		"x ∉ dom xs ∧ y ∉ dom ys",
	}
	got := make([]csdf.Predicate, 0, len(result.Diagram.Edges))
	for _, edge := range result.Diagram.Edges {
		got = append(got, edge.Guard)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("guards mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandRefusesToSyncAMapWithItself(t *testing.T) {
	// Arrange: two instances of one map cannot take an event together, because
	// one edge cannot say twice what the map becomes.
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(買い手)
sync TRANSFER : accounts(買い手), accounts(売り手)
@enduml
`)
	load := stubLoader(map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : TRANSFER
@enduml
`})

	want := []string{`sync TRANSFER: the map "accounts" is synced with itself; one edge cannot say twice what a map becomes`}
	if diff := cmp.Diff(want, diagnosticsOf(t, global, load, promote.SeverityError)); diff != "" {
		t.Errorf("errors mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandRefusesToSyncTau(t *testing.T) {
	// Arrange: an internal event is by definition not shared, and a synced one
	// would have to carry the instance ids as arguments.
	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
sync tau : accounts(口座ID)
@enduml
`)
	load := stubLoader(map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : tau
@enduml
`})

	want := []string{"sync tau: an internal event cannot be synchronised"}
	if diff := cmp.Diff(want, diagnosticsOf(t, global, load, promote.SeverityError)); diff != "" {
		t.Errorf("errors mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandReportsATemplateThatCannotRender(t *testing.T) {
	// Arrange: a template that parses but cannot run would otherwise leave its
	// own error inside an opaque predicate, where nothing downstream can catch
	// it.
	templates, err := promote.ParseTemplates(`{{define "at"}}{{.Nope.Missing}}{{end}}`)
	if err != nil {
		t.Fatalf("ParseTemplates() = _, %v; want no error", err)
	}

	global := mustParseGlobal(`@startuml
state "稼働中" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`)
	load := stubLoader(map[string]string{"local/ACCOUNT.puml": `@startuml
state "未開設" as none
state "開設済み" as open
[*] --> none
none --> open : OPEN
open --> open : TOUCH
@enduml
`})

	// Act
	_, diags := promote.Expand(global, load, promote.Options{Templates: templates})

	// Assert: the same broken clause is used by every edge, but it is one fault.
	errs := promote.Errors(diags)
	if len(errs) != 1 {
		t.Fatalf("Expand() reported %d errors, want 1: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, `template "at"`) {
		t.Errorf("Expand() error = %q; want it to name the clause", errs[0].Message)
	}
}
