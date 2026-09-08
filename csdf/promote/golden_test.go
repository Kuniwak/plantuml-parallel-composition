package promote_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the recorded expansions")

// TestExpandMatchesTheRecordedExpansions expands every worked example and
// compares it with the expansion recorded next to it. The recordings are what
// makes a change in the generated wording visible in review.
//
// Run with -update to rewrite them.
func TestExpandMatchesTheRecordedExpansions(t *testing.T) {
	const dir = "../../examples/promote"

	paths, err := filepath.Glob(filepath.Join(dir, "*.puml"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	for _, path := range paths {
		if strings.HasSuffix(path, ".expanded.puml") {
			continue
		}

		t.Run(filepath.Base(path), func(t *testing.T) {
			g, err := promote.LoadGlobal(path)
			if err != nil {
				t.Fatalf("promote.LoadGlobal(%q) error = %v", path, err)
			}

			x, diags, err := promote.Expand(g, promote.FileLoader(filepath.Dir(path)))
			if err != nil {
				t.Fatalf("promote.Expand(%q) error = %v", path, err)
			}
			for _, d := range diags {
				if d.Severity == promote.SeverityError {
					t.Fatalf("promote.Expand(%q) diagnostic = %v", path, d)
				}
			}

			golden := strings.TrimSuffix(path, ".puml") + ".expanded.puml"
			got := x.String()
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatalf("WriteFile(%q) error = %v", golden, err)
				}
				return
			}

			bs, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v; run the test with -update to record it", golden, err)
			}
			if diff := cmp.Diff(string(bs), got); diff != "" {
				t.Errorf("promote.Expand(%q) mismatch (-recorded +got):\n%s", path, diff)
			}
		})
	}
}
