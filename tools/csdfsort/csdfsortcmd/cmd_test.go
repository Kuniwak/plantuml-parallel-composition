package csdfsortcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOK(t *testing.T) {
	type testCase struct {
		Args  []string
		Stdin string
		Want  string
	}

	testCases := map[string]testCase{
		"a file argument (a hand-written diagram in authoring order)": {
			Args: []string{filepath.Join("testdata", "unsorted.puml")},
			Want: `@startuml
state "First" as s0
state "Second" as s1
[*] --> s1
s0 --> s1 : a
s1 --> s0 : a ; g
s1 --> s0 : b
@enduml
`,
		},
		"standard input (equivalent to a file argument)": {
			Args: []string{},
			Stdin: `@startuml
state "s1" as s1
state "s0" as s0
[*] --> s0
s0 --> s1 : a
@enduml
`,
			Want: `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`,
		},
		"-v (representative value)": {
			Args: []string{"-v"},
			Want: version.Version + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(testCase.Want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncReportsParseErrors(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("not a diagram\n"))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Error("want non-zero exit status, got 0")
	}
}

func TestNewMainFuncRejectsDirectives(t *testing.T) {
	// Arrange: a diagram whose behaviour is still hidden in its directives is
	// not one this tool may act on.
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(`@startuml
state "running" as running
running : accounts
[*] --> running
promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
`))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Errorf("want a non-zero exit status, got 0")
	}
	if !strings.Contains(spy.Stderr.String(), "csdfpromote") {
		t.Errorf("want stderr to name csdfpromote, got %q", spy.Stderr.String())
	}
}
