package csdfpromotecmd

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

const accountsPath = "../../../examples/promote/ACCOUNTS.puml"

func TestNewMainFuncExpandsTheRecordedExample(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	exitStatus := cmdFunc([]string{accountsPath}, spy.New())

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}

	bs, err := os.ReadFile("../../../examples/promote/ACCOUNTS.expanded.puml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	// The recorded expansion keeps the diagram's own name; the command replaces
	// it with the command line that produced the file.
	want := strings.Replace(
		string(bs),
		"@startuml ACCOUNTS\n",
		"@startuml auto-generated-by: csdfpromote "+accountsPath+"\n",
		1)
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncOmitsTheOriginComments(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	exitStatus := cmdFunc([]string{"-no-comments", accountsPath}, spy.New())

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	if strings.Contains(spy.Stdout.String(), "' promote:") {
		t.Errorf("want no origin comments, got:\n%s", spy.Stdout.String())
	}
}

func TestNewMainFuncLintOnlyPrintsNothing(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	exitStatus := cmdFunc([]string{"-lint-only", accountsPath}, spy.New())

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	if got := spy.Stdout.String(); got != "" {
		t.Errorf("want no output, got %q", got)
	}
}

func TestNewMainFuncRefusesAnUnsoundPromotion(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// The map is not a state variable of the state its block was written in.
	spy.Stdin = strings.NewReader(`@startuml A
state "稼働中" as running {
  running : audits ; 監査ID ⇸ Audit

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`)

	exitStatus := cmdFunc([]string{"-base", "../../../examples/promote"}, spy.New())

	if exitStatus != 1 {
		t.Fatalf("want 1, got %d", exitStatus)
	}
	if got := spy.Stdout.String(); got != "" {
		t.Errorf("want no output, got %q", got)
	}
	if !strings.Contains(spy.Stderr.String(), `holds no state variable named "accounts"`) {
		t.Errorf("want the reason on stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncJSONReportsTheDirectives(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	exitStatus := cmdFunc([]string{"-json", accountsPath}, spy.New())

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stdout.String(), `"map":"accounts"`) {
		t.Errorf("want the promotion in the JSON, got %q", spy.Stdout.String())
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	exitStatus := cmdFunc([]string{"-v"}, spy.New())

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(version.Version+"\n", spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
