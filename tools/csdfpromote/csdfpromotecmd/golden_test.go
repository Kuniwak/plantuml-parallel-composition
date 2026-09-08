package csdfpromotecmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfpromote/csdfpromotecmd"
	"github.com/google/go-cmp/cmp"
)

// TestGoldenExamples expands every example under examples/promote and compares
// the result with the recorded expansion next to it. Run with -update to
// rewrite the recordings.
func TestGoldenExamples(t *testing.T) {
	const examplesDir = "../../../examples/promote"

	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", examplesDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".puml" || strings.HasSuffix(name, ".expanded.puml") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			inputPath := filepath.Join(examplesDir, name)
			goldenPath := filepath.Join(examplesDir, strings.TrimSuffix(name, ".puml")+".expanded.puml")

			cmdFunc := tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc())
			spy := cli.SpyProcInout()

			exitStatus := cmdFunc([]string{inputPath}, spy.New())
			if exitStatus != 0 {
				t.Fatalf("want exit status 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
			}
			if spy.Stderr.Len() != 0 {
				t.Errorf("want empty stderr, got %q", spy.Stderr.String())
			}

			// The recorded expansion must not name the path it was produced
			// from, or it would differ per checkout layout.
			got := strings.Replace(spy.Stdout.String(),
				"auto-generated-by: csdfpromote "+inputPath,
				"auto-generated-by: csdfpromote "+name, 1)

			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatalf("cannot write %s: %v", goldenPath, err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("cannot read %s (run the test with UPDATE_GOLDEN=1 to record it): %v", goldenPath, err)
			}
			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("%s mismatch (-want +got):\n%s", goldenPath, diff)
			}
		})
	}
}
