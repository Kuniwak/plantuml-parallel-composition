package promote_test

import (
	"strings"
	"testing"
)

// The two families take BOOK together: booking a buy trade is the same event as
// counting it into the segregation report of its business day.
var syncLocals = map[string]string{
	"local/BUY.puml": `@startuml BUY
state "なし" as buyNone
state "約定済み" as buyBooked
state "記帳済み" as buyPosted
[*] --> buyNone
buyNone --> buyBooked : BUY(数量)
buyBooked --> buyPosted : BOOK(数量)
buyPosted --> buyNone : SETTLE
@enduml
`,
	"local/CYCLE.puml": `@startuml CYCLE
state "未開始" as cycIdle
state "集計中" as cycCounting
state "確定済み" as cycFixed
[*] --> cycIdle
cycIdle --> cycCounting : BOOK(数量)
cycCounting --> cycCounting : BOOK(数量)
cycCounting --> cycFixed : tau
cycFixed --> cycIdle : REPORT
@enduml
`,
}

const syncGlobal = `@startuml TRADES
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
@enduml
`

func TestExpandMergesTheSyncedEdges(t *testing.T) {
	x, diags := expand(t, syncGlobal, syncLocals)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}

	// BOOK has one edge on the buys side and two on the cycles side, so the
	// product is two merged edges and neither side keeps a BOOK of its own.
	want := []string{
		"running --> running : BOOK(約定ID, 基準日, 数量) ; 約定ID ∈ dom buys ∧ buys(約定ID) ∈ 〈約定済み〉 ∧ 基準日 ∈ dom cycles ∧ cycles(基準日) ∈ 〈集計中〉 ; buys' = buys ⊕ {約定ID ↦ 〈記帳済み〉} ∧ cycles' = cycles ⊕ {基準日 ↦ 〈集計中〉}",
		"running --> running : BOOK(約定ID, 基準日, 数量) ; 約定ID ∈ dom buys ∧ buys(約定ID) ∈ 〈約定済み〉 ∧ 基準日 ∉ dom cycles ; buys' = buys ⊕ {約定ID ↦ 〈記帳済み〉} ∧ cycles' = cycles ∪ {基準日 ↦ 〈集計中〉}",
	}

	got := x.String()
	for _, line := range want {
		if !strings.Contains(got, line) {
			t.Errorf("promote.Expand() missing line:\n%s\ngot:\n%s", line, got)
		}
	}
	if n := strings.Count(got, " : BOOK("); n != len(want) {
		t.Errorf("promote.Expand() has %d BOOK edges, want %d:\n%s", n, len(want), got)
	}
}

func TestExpandPromotesTauWithoutAnInstanceID(t *testing.T) {
	x, _ := expand(t, syncGlobal, syncLocals)

	want := "running --> running : tau ; 基準日 ∈ dom cycles ∧ cycles(基準日) ∈ 〈集計中〉 ; cycles' = cycles ⊕ {基準日 ↦ 〈確定済み〉} ∧ buys' = buys"
	if got := x.String(); !strings.Contains(got, want) {
		t.Errorf("promote.Expand() missing line:\n%s\ngot:\n%s", want, got)
	}
}

func TestExpandConjoinsTheConstrainGuards(t *testing.T) {
	source := strings.Replace(syncGlobal, "@enduml", `note bottom of runningBuys
  constrain BUY(約定ID, 数量) ; 数量 は最小取引単位の倍数である
end note

note as cons2
  constrain BOOK(約定ID, 基準日, 数量) ; 基準日 は 約定ID の約定日である
end note
@enduml`, 1)

	x, diags := expand(t, source, syncLocals)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}

	got := x.String()
	if !strings.Contains(got, "∧ 数量 は最小取引単位の倍数である ;") {
		t.Errorf("promote.Expand() did not constrain the creation edge:\n%s", got)
	}
	if n := strings.Count(got, "基準日 は 約定ID の約定日である"); n != 2 {
		t.Errorf("promote.Expand() constrained %d of the 2 merged BOOK edges:\n%s", n, got)
	}
}

func TestExpandRefusesADirectiveThatCannotMeanAnything(t *testing.T) {
	cases := map[string]struct {
		global string
		want   string
	}{
		"a sync of a map that is not promoted": {
			global: strings.Replace(syncGlobal, "sync BOOK : buys(約定ID), cycles(基準日)", "sync BOOK : buys(約定ID), fees(手数料ID)", 1),
			want:   `error: line 17: "fees" is not promoted anywhere`,
		},
		"a sync of an event no local diagram has": {
			global: strings.Replace(syncGlobal, "sync BOOK : buys(約定ID), cycles(基準日)", "sync PRICE : buys(約定ID), cycles(基準日)", 1),
			want:   `error: line 17: local/BUY.puml has no edge on "PRICE"`,
		},
		"a sync of tau": {
			global: strings.Replace(syncGlobal, "sync BOOK : buys(約定ID), cycles(基準日)", "sync tau : buys(約定ID), cycles(基準日)", 1),
			want:   `error: line 17: tau cannot be synced; an internal event two families take together is an observable one`,
		},
		"a sync that names one map twice": {
			global: strings.Replace(syncGlobal, "sync BOOK : buys(約定ID), cycles(基準日)", "sync BOOK : buys(約定ID), buys(約定ID2)", 1),
			want:   `error: line 17: this sync names "buys" twice; one event cannot move two instances of one map at once`,
		},
		"a constrain that matches no edge": {
			global: strings.Replace(syncGlobal, "@enduml", `note as c1
  constrain BUY(約定ID) ; 数量 は最小取引単位の倍数である
end note
@enduml`, 1),
			want: `error: line 20: no expanded edge is "BUY" with 1 argument`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, c.global, syncLocals)
			if x != nil {
				t.Errorf("promote.Expand() expansion = %q, want none", x.String())
			}
			if !hasDiagnostic(diags, c.want) {
				t.Errorf("promote.Expand() diagnostics = %v, want one of them to be %q", diags, c.want)
			}
		})
	}
}
