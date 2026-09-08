package csdfpromotecmd

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
)

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		global, err := csdf.ParseBytesAllowingDirectives(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}

		result, diags := promote.Expand(global, promote.Loader(csdf.NewFileDiagramLoader(opts.BaseDir)))

		warnings := 0
		for _, diag := range diags {
			fmt.Fprintln(inout.Stderr, diag)
			if diag.Severity == promote.SeverityWarning {
				warnings++
			}
		}
		// An unsound expansion must not reach the tools downstream, so nothing
		// is printed when the check failed.
		if errs := promote.Errors(diags); len(errs) > 0 {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", errors.New(pluralize(len(errs), "error")+" in the promotion directives"))
		}
		if opts.Werror && warnings > 0 {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", errors.New(pluralize(warnings, "warning")+" in the promotion directives (-Werror)"))
		}

		if opts.LintOnly {
			return nil
		}

		result.Diagram.Name = tools.GeneratedBy("csdfpromote", opts.Args)
		origins := result.Origins
		if opts.NoComments {
			origins = nil
		}
		fmt.Fprint(inout.Stdout, result.Diagram.StringWithEdgeComments(origins))
		return nil
	}
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
