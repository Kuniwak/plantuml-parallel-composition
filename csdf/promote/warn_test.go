package promote_test

import (
	"strings"
	"testing"
)

func TestExpandWarnsAboutWhatIsProbablyNotMeant(t *testing.T) {
	cases := map[string]struct {
		global string
		locals map[string]string
		want   string
		// wantCount is how many times the diagnostic must appear, when saying
		// it once per directive is the point. Zero means at least once.
		wantCount int
	}{
		"a post on the local start edge": {
			global: syncGlobal,
			locals: withLocal(syncLocals, "local/BUY.puml", `@startuml BUY
state "なし" as buyNone
state "約定済み" as buyBooked
[*] --> buyNone : 数量は 0
buyNone --> buyBooked : BUY(数量)
buyBooked --> buyNone : BOOK(数量)
@enduml
`),
			want: `warning: line 6: the start edge of local/BUY.puml has a post, which says nothing: its state means that the instance does not exist. Leave it out`,
		},
		"a post on a deletion edge": {
			global: syncGlobal,
			locals: withLocal(syncLocals, "local/BUY.puml", `@startuml BUY
state "なし" as buyNone
state "約定済み" as buyBooked
[*] --> buyNone
buyNone --> buyBooked : BUY(数量)
buyBooked --> buyNone : BOOK(数量) ; true ; 数量は 0
@enduml
`),
			want: `warning: line 6: the post of BOOK(数量) in local/BUY.puml is dropped: the edge deletes the instance it would be about. Write an effect on another map as a sync or a hand-written edge`,
		},
		"an event two families have without a sync": {
			global: strings.Replace(syncGlobal, `note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)
end note
`, "", 1),
			locals: syncLocals,
			want:   `warning: local/BUY.puml and local/CYCLE.puml both have an edge on "BOOK", which is not synced; each family takes it on its own`,
		},
		"a note pointing at a map it does not mention": {
			global: strings.Replace(syncGlobal, `note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)`, `note bottom of runningCycles
  sync BOOK : buys(約定ID)`, 1),
			locals: syncLocals,
			want:   `warning: line 17: this note points at "runningCycles", which is not a block of any map it names`,
		},
		"a constrain guard that names no argument": {
			global: strings.Replace(syncGlobal, "@enduml", `note as c1
  constrain BUY(約定ID, 数量) ; 相場は開いている
end note
@enduml`, 1),
			locals: syncLocals,
			want:   `warning: line 20: the guard of this constrain names none of its arguments, so it says nothing about the instance`,
		},
		"a type the state variable does not mention": {
			global: strings.Replace(syncGlobal, `running : buys ; 約定ID ⇸ Buy`, `running : buys ; 約定ID ⇸ Sell`, 1),
			locals: syncLocals,
			want:   `warning: line 6: "buys" is promoted as "Buy" but its state variable is written "約定ID ⇸ Sell"`,
		},
		// BOOK is merged from two combinations, so the directive matches two
		// edges. The mistyped argument is one mistake and is said once.
		"a constrain argument no matching edge has": {
			global: strings.Replace(syncGlobal, "@enduml", `note as c1
  constrain BOOK(約定ID, 基準日, zzz) ; zzz は最小取引単位の倍数である
end note
@enduml`, 1),
			locals:    syncLocals,
			want:      `warning: line 20: BOOK has no argument named "zzz"; the guard says nothing about it`,
			wantCount: 1,
		},
		"a sync whose sides carry different arities": {
			global: syncGlobal,
			locals: withLocal(syncLocals, "local/CYCLE.puml", `@startuml CYCLE
state "未開始" as cycIdle
state "集計中" as cycCounting
[*] --> cycIdle
cycIdle --> cycCounting : BOOK
cycCounting --> cycCounting : BOOK(数量)
@enduml
`),
			want: `warning: line 17: the edges of "cycles" on "BOOK" do not all take the same arguments, so the merged event does not either`,
		},
		"an !include outside a <<promote>> block": {
			global: strings.Replace(syncGlobal, "[*] --> running", "!include local/BUY.puml\n[*] --> running", 1),
			locals: syncLocals,
			want:   `info: line 14: this !include is not inside a <<promote>> block, so it names no local diagram and is dropped`,
		},
		"a directive whose name is not spelled as one": {
			global: strings.Replace(syncGlobal, "  sync BOOK :", "  Sync BOOK :", 1),
			locals: syncLocals,
			want:   `warning: line 17: this note starts with "Sync", which is not a directive; write "sync" to make it one`,
		},
		"a map that is frozen in a state": {
			global: strings.Replace(syncGlobal, "@enduml", `state "保守中" as maintenance
maintenance : buys ; 約定ID ⇸ Buy
maintenance : cycles ; 基準日 ⇸ Cycle
running --> maintenance : MAINT ; true ; buys' = buys ∧ cycles' = cycles
@enduml`, 1),
			locals: syncLocals,
			want:   `info: state "maintenance" holds "buys" but has no <<promote>> block for it, so the family is frozen there`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, c.global, c.locals)
			if x == nil {
				t.Fatalf("promote.Expand() expansion = nil; a warning must not stop it: %v", diags)
			}
			if got := countDiagnostic(diags, c.want); got == 0 || (c.wantCount != 0 && got != c.wantCount) {
				t.Errorf("promote.Expand() has %d diagnostics reading %q, want %d; got %v", got, c.want, max(c.wantCount, 1), diags)
			}
		})
	}
}

func withLocal(locals map[string]string, path, source string) map[string]string {
	out := map[string]string{path: source}
	for k, v := range locals {
		if k != path {
			out[k] = v
		}
	}
	return out
}
