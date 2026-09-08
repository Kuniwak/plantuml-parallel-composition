// Package rendercheck renders the hand-written global diagrams through PlantUML.
//
// A global diagram spells its promotion directives in PlantUML's own syntax -
// composite states, !include and notes - so that the diagram an author writes is
// also the picture a reader looks at. Nothing in the Go tests would notice if a
// spelling stopped rendering, so PlantUML itself is asked. The expanded form is
// not checked: it is one state with a bundle of self-loops and is never drawn.
//
// PlantUML is not vendored, so the check is skipped unless it is available:
//
//	CSDF_PLANTUML          the plantuml executable (default: "plantuml" on PATH)
//	CSDF_REQUIRE_PLANTUML  set to a non-empty value to fail rather than skip
//
// CI sets CSDF_REQUIRE_PLANTUML in the job that installs PlantUML, so a runner
// that lost it fails instead of reporting a pass having checked nothing.
package rendercheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	envPlantUML = "CSDF_PLANTUML"
	envRequire  = "CSDF_REQUIRE_PLANTUML"
)

// renderTimeout bounds one PlantUML invocation. A diagram of this size renders
// in about a second, so this is slack, not a budget.
const renderTimeout = 2 * time.Minute

// examplesDir holds the hand-written global diagrams. Its local/ subdirectory
// holds the local diagrams they !include, which are checked through their
// includers rather than on their own.
const examplesDir = "../../../examples/promote"

func TestGlobalDiagramsRender(t *testing.T) {
	bin := plantUML(t)

	paths, err := filepath.Glob(filepath.Join(examplesDir, "*.puml"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no global diagrams under %s", examplesDir)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
			defer cancel()

			// -checkonly reports a malformed diagram through the exit status;
			// without it PlantUML writes the error into the image and exits 0.
			cmd := exec.CommandContext(ctx, bin, "-checkonly", filepath.Base(path))
			cmd.Dir = filepath.Dir(path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("plantuml -checkonly %s error = %v\n%s", path, err, out)
			}
		})
	}
}

// plantUML returns the executable to run, skipping the test when there is none
// and CSDF_REQUIRE_PLANTUML does not insist on one.
func plantUML(t *testing.T) string {
	t.Helper()

	bin := os.Getenv(envPlantUML)
	if bin == "" {
		bin = "plantuml"
	}
	if _, err := exec.LookPath(bin); err != nil {
		if os.Getenv(envRequire) != "" {
			t.Fatalf("%s is set but %s is not runnable: %v", envRequire, bin, err)
		}
		t.Skipf("%s not found; set %s to point at it", bin, envPlantUML)
	}
	return bin
}
