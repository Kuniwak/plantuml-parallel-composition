package csdfpromotecmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

const globalWithDirectives = `@startuml
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
[*] --> running
promote ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`

func TestNewMainFuncExpandsDirectives(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(globalWithDirectives))

	// Act
	exitStatus := cmdFunc([]string{"-base", "testdata"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Fatalf("want exit status 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	want := `@startuml auto-generated-by: csdfpromote -base testdata
state "稼働中" as running
running: accounts ; 口座ID ⇸ Account
[*] --> running
' promote: ACCOUNT.puml 〈開設済み〉 → 〈未開設〉
running --> running : CLOSE(口座ID) ; 口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ; accounts' = {口座ID} ⩤ accounts
' promote: ACCOUNT.puml 〈未開設〉 → 〈開設済み〉
running --> running : OPEN(口座ID) ; 口座ID ∉ dom accounts ; accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}
@enduml
`
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncSuppressesComments(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(globalWithDirectives))

	exitStatus := cmdFunc([]string{"-base", "testdata", "-no-comments"}, spy.New())

	if exitStatus != 0 {
		t.Fatalf("want exit status 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if strings.Contains(spy.Stdout.String(), "' promote:") {
		t.Errorf("-no-comments still printed origin comments:\n%s", spy.Stdout.String())
	}
}

func TestNewMainFuncLintOnlyPrintsNothing(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(globalWithDirectives))

	exitStatus := cmdFunc([]string{"-base", "testdata", "-lint-only"}, spy.New())

	if exitStatus != 0 {
		t.Fatalf("want exit status 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stdout.Len() != 0 {
		t.Errorf("-lint-only wrote to stdout: %q", spy.Stdout.String())
	}
}

func TestNewMainFuncReportsLintErrors(t *testing.T) {
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(`@startuml
state "稼働中" as running
[*] --> running
promote ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`))

	exitStatus := cmdFunc([]string{"-base", "testdata"}, spy.New())

	if exitStatus == 0 {
		t.Errorf("want a non-zero exit status, got 0")
	}
	if !strings.Contains(spy.Stderr.String(), "no state variable") {
		t.Errorf("want stderr to explain the missing map, got %q", spy.Stderr.String())
	}
	if spy.Stdout.Len() != 0 {
		t.Errorf("want no output for a diagram that failed the check, got %q", spy.Stdout.String())
	}
}
