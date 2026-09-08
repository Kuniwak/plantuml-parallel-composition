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

// The command's other runs differ only in what they print, so they are one
// table: the arguments, the exit status, and what standard output has to say.
func TestNewMainFunc(t *testing.T) {
	cases := map[string]struct {
		args            []string
		stdin           string
		wantExit        int
		wantStdoutEmpty bool
		wantStdout      []string
		wantNoStdout    []string
		wantStderr      string
	}{
		"-no-comments leaves the origins out": {
			args:         []string{"-no-comments", accountsPath},
			wantNoStdout: []string{"' promote:"},
		},
		"-lint-only prints nothing": {
			args:            []string{"-lint-only", accountsPath},
			wantStdoutEmpty: true,
		},
		"-json reports the directives": {
			args:       []string{"-json", accountsPath},
			wantStdout: []string{`"map":"accounts"`},
		},
		"-template rewords the clauses": {
			args:       []string{"-template", "../../../examples/promote/templates/ja.tmpl", accountsPath},
			wantStdout: []string{"口座ID は accounts にない"},
		},
		"-v prints the version": {
			args:       []string{"-v"},
			wantStdout: []string{version.Version + "\n"},
		},
		// The map is not a state variable of the state its block was written in.
		"an unsound promotion is refused, with the reason and no diagram": {
			args: []string{"-base", "../../../examples/promote"},
			stdin: `@startuml A
state "稼働中" as running {
  running : audits ; 監査ID ⇸ Audit

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      `holds no state variable named "accounts"`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader(c.stdin)

			exitStatus := cmdFunc(c.args, spy.New())

			if exitStatus != c.wantExit {
				t.Log(spy.Stderr.String())
				t.Fatalf("want %d, got %d", c.wantExit, exitStatus)
			}

			stdout := spy.Stdout.String()
			if c.wantStdoutEmpty && stdout != "" {
				t.Errorf("want no output, got %q", stdout)
			}
			for _, want := range c.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("want %q in the output, got:\n%s", want, stdout)
				}
			}
			for _, unwanted := range c.wantNoStdout {
				if strings.Contains(stdout, unwanted) {
					t.Errorf("want no %q in the output, got:\n%s", unwanted, stdout)
				}
			}
			if c.wantStderr != "" && !strings.Contains(spy.Stderr.String(), c.wantStderr) {
				t.Errorf("want %q on stderr, got %q", c.wantStderr, spy.Stderr.String())
			}
		})
	}
}
