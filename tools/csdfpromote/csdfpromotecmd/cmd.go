package csdfpromotecmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
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

		global, err := promote.ParseGlobalBytes(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}

		if opts.JSON {
			if err := json.NewEncoder(inout.Stdout).Encode(global); err != nil {
				return fmt.Errorf("csdfpromotecmd.NewMainFunc: writing JSON: %w", err)
			}
			return nil
		}

		templates := promote.DefaultTemplates()
		if opts.Template != "" {
			loaded, err := promote.LoadTemplates(opts.Template)
			if err != nil {
				return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
			}
			templates = loaded
		}

		expansion, diags := promote.Expand(global, promote.FileLoader(opts.Base), templates)
		for _, d := range diags {
			fmt.Fprintln(inout.Stderr, d)
		}
		// An error leaves the diagram unprinted, so an unsound expansion never
		// reaches the tools downstream.
		if err := promote.Refusal(diags, opts.Werror); err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}
		if opts.LintOnly {
			return nil
		}

		expansion.Diagram.Name = tools.GeneratedBy("csdfpromote", opts.Args)
		if opts.NoComments {
			fmt.Fprint(inout.Stdout, expansion.StringWithoutComments())
			return nil
		}
		fmt.Fprint(inout.Stdout, expansion.String())
		return nil
	}
}
