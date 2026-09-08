package csdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiagramReadsPlantUMLPNG(t *testing.T) {
	p := filepath.Join("..", "examples", "valid", "client.png")
	bs, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("TestLoadDiagramReadsPlantUMLPNG: cannot read file: %q", p)
	}

	diagram, err := ParseBytes(bs)
	if err != nil {
		t.Fatalf("LoadDiagram() error = %v", err)
	}
	if len(diagram.States) == 0 {
		t.Fatal("LoadDiagram() returned a diagram without states")
	}
}

func TestParseHintsAtCsdfpromoteWhenTheSourceStillHoldsDirectives(t *testing.T) {
	cases := map[string]string{
		"a <<promote>> block": `@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
		"a sync note": `@startuml A
state "稼働中" as running
running : accounts
[*] --> running
note as n1
  sync BOOK : accounts(口座ID)
end note
@enduml
`,
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil {
				t.Fatal("Parse() error = nil, want a parse error")
			}
			if !strings.Contains(err.Error(), "csdfpromote") {
				t.Errorf("Parse() error = %q, want it to name csdfpromote", err)
			}
		})
	}
}

func TestParseDoesNotHintAtCsdfpromoteForAnOrdinaryMistake(t *testing.T) {
	cases := map[string]string{
		"a malformed edge": `@startuml A
state "稼働中" as running
[*] --> running
running --> ; NO-EVENT
@enduml
`,
		// A local diagram wraps its PlantUML-only lines in CSDF-IGNORE, which
		// is exactly where an !include of a skin or a theme goes. What is
		// ignored cannot be a directive.
		"an !include inside a CSDF-IGNORE region": `@startuml A
' CSDF-IGNORE-BEGIN
!include style/theme.puml
' CSDF-IGNORE-END
state "稼働中" as running
[*] --> running
running --> ; NO-EVENT
@enduml
`,
		// A note is PlantUML's, not promotion's. The advice for one that was
		// not wrapped is CSDF-IGNORE, not csdfpromote.
		"a plain note": `@startuml A
state "稼働中" as running
[*] --> running
note left of running
  この図は promotion とは関係がない
end note
@enduml
`,
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil {
				t.Fatal("Parse() error = nil, want a parse error")
			}
			if strings.Contains(err.Error(), "csdfpromote") {
				t.Errorf("Parse() error = %q, want it not to name csdfpromote", err)
			}
		})
	}
}

// The hint is added to the error, not put in place of it, so a caller can still
// tell what the parser actually said.
func TestParseKeepsTheParseErrorUnderTheHint(t *testing.T) {
	_, err := Parse(`@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`)
	if err == nil {
		t.Fatal("Parse() error = nil, want a parse error")
	}
	var hinted *PromotionHintError
	if !errors.As(err, &hinted) {
		t.Fatalf("Parse() error = %v, want a *PromotionHintError in the chain", err)
	}
	if errors.Unwrap(hinted) == nil {
		t.Error("PromotionHintError wraps nothing, want the parse error under it")
	}
}
