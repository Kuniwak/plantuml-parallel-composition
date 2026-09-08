package csdfparsecmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncPrintsJSON(t *testing.T) {
	// Arrange
	input := `@startuml
state "Initial" as s0
s0: ready ; bool
s0: count
state "Done" as s1
[*] --> s0 : initialize
s0 --> s1 : finish(result) ; ready ; done
s1 --> [*] : complete
@enduml
`
	want := `{"states":{"s0":{"name":"Initial","vars":[{"name":"ready","type":"bool"},{"name":"count"}],"line":2},"s1":{"name":"Done","vars":[],"line":5}},"start_edge":{"dst":"s0","post":"initialize","line":6},"edges":[{"src":"s0","dst":"s1","event":"finish(result)","guard":"ready","post":"done","line":7}],"end_edge":{"src":"s1","guard":"complete","line":8},"promotes":[],"syncs":[],"constrains":[]}` + "\n"

	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncReadsFileArgument(t *testing.T) {
	// Arrange: `csdfparse <file>` must be equivalent to reading from stdin.
	want := `{"states":{"s0":{"name":"SKIP","vars":[],"line":3}},"start_edge":{"dst":"s0","post":"true","line":5},"edges":[],"end_edge":{"src":"s0","guard":"true","line":6},"promotes":[],"syncs":[],"constrains":[]}` + "\n"
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"../../../examples/valid/skip.puml"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"-v"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	want := version.Version + "\n"
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncPrintsDirectives(t *testing.T) {
	// Arrange: csdfparse is the one tool that reports directives instead of
	// refusing them, because reporting the diagram is all it does.
	input := `@startuml
state "running" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID) in running
sync EVT-BOOK : accounts(口座ID)
constrain EVT-OPEN(口座ID) ; 未開設
@enduml
`
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Errorf("want exit status 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	got := spy.Stdout.String()
	for _, want := range []string{
		`"promotes":[{"path":"local/ACCOUNT.puml","type":"Account","map":"accounts","id_param":"口座ID","in":["running"],"line":5}]`,
		`"syncs":[{"event":"EVT-BOOK","targets":[{"map":"accounts","param":"口座ID"}],"line":6}]`,
		`"constrains":[{"event":"EVT-OPEN","params":["口座ID"],"guard":"未開設","line":7}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout does not contain %s\ngot: %s", want, got)
		}
	}
}
