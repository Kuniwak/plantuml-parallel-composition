package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func TestExpandRendersTheClausesWithTheGivenTemplates(t *testing.T) {
	tmpl, err := promote.LoadTemplates("../../examples/promote/templates/ja.tmpl")
	if err != nil {
		t.Fatalf("promote.LoadTemplates() error = %v", err)
	}

	g, err := promote.ParseGlobal(syncGlobal)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	x, _ := promote.Expand(g, promote.MapLoader(syncLocals), tmpl)

	want := "running --> running : SETTLE(約定ID) ; 約定ID は buys にある、buys の 約定ID は〈記帳済み〉 ; buys から 約定ID を除く、cycles は変わらない"
	if got := x.String(); !strings.Contains(got, want) {
		t.Errorf("promote.Expand() missing line:\n%s\ngot:\n%s", want, got)
	}
}

func TestLoadTemplatesRefusesOneThatCannotRender(t *testing.T) {
	if _, err := promote.LoadTemplates("../../examples/promote/testdata/broken.tmpl"); err == nil {
		t.Fatal("promote.LoadTemplates() error = nil, want a refusal")
	}
}
