package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func TestExpandWarnsAboutWhatIsProbablyNotMeant(t *testing.T) {
	cases := map[string]struct {
		global string
		locals map[string]string
		want   string
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
			want: `warning: local/BUY.puml and local/CYCLE.puml both have an edge on "BOOK", which is not synced; each family takes it on its own`,
		},
		"a note pointing at a map it does not mention": {
			global: strings.Replace(syncGlobal, `note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)`, `note bottom of runningCycles
  sync BOOK : buys(約定ID)`, 1),
			want: `warning: line 17: this note points at "runningCycles", which is not a block of any map it names`,
		},
		"a constrain guard that names no argument": {
			global: strings.Replace(syncGlobal, "@enduml", `note as c1
  constrain BUY(約定ID, 数量) ; 相場は開いている
end note
@enduml`, 1),
			want: `warning: line 20: the guard of this constrain names none of its arguments, so it says nothing about the instance`,
		},
		"a type the state variable does not mention": {
			global: strings.Replace(syncGlobal, `running : buys ; 約定ID ⇸ Buy`, `running : buys ; 約定ID ⇸ Sell`, 1),
			want:   `warning: line 6: "buys" is promoted as "Buy" but its state variable is written "約定ID ⇸ Sell"`,
		},
		"a map that is frozen in a state": {
			global: strings.Replace(syncGlobal, "@enduml", `state "保守中" as maintenance
maintenance : buys ; 約定ID ⇸ Buy
maintenance : cycles ; 基準日 ⇸ Cycle
running --> maintenance : MAINT ; true ; buys' = buys ∧ cycles' = cycles
@enduml`, 1),
			want: `info: state "maintenance" holds "buys" but has no <<promote>> block for it, so the family is frozen there`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			locals := c.locals
			if locals == nil {
				locals = syncLocals
			}

			g, err := promote.ParseGlobal(c.global)
			if err != nil {
				t.Fatalf("promote.ParseGlobal() error = %v", err)
			}

			x, diags, err := promote.Expand(g, loaderOf(locals))
			if err != nil {
				t.Fatalf("promote.Expand() error = %v", err)
			}
			if x == nil {
				t.Fatalf("promote.Expand() expansion = nil; a warning must not stop it: %v", diags)
			}
			if !hasDiagnostic(diags, c.want) {
				t.Errorf("promote.Expand() diagnostics = %v, want one of them to be %q", diags, c.want)
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
