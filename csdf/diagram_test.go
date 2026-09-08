package csdf

import (
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
	_, err := Parse(`@startuml A
state "稼働中" as running
[*] --> running
running --> ; NO-EVENT
@enduml
`)
	if err == nil {
		t.Fatal("Parse() error = nil, want a parse error")
	}
	if strings.Contains(err.Error(), "csdfpromote") {
		t.Errorf("Parse() error = %q, want it not to name csdfpromote", err)
	}
}
