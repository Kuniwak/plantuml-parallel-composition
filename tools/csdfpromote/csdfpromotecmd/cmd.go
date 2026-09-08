package csdfpromotecmd

import (
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

		templates, err := promote.LoadTemplates(opts.TemplatePath)
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}

		result, diags, err := promote.Run(global, csdf.NewFileDiagramLoader(opts.BaseDir), promote.RunOptions{
			Templates: templates,
			Werror:    opts.Werror,
		})
		for _, diag := range diags {
			fmt.Fprintln(inout.Stderr, diag)
		}
		// An unsound expansion must not reach the tools downstream, so nothing
		// is printed when the check failed.
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}

		if opts.LintOnly {
			return nil
		}

		result.Diagram.Name = tools.GeneratedBy("csdfpromote", opts.Args)
		fmt.Fprint(inout.Stdout, result.String(!opts.NoComments))
		return nil
	}
}
