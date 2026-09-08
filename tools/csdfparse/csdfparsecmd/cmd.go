package csdfparsecmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
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

		// Directives are reported, not refused: printing the diagram is all
		// this tool does, and a diagram that still carries them is exactly what
		// its author wants to inspect.
		diagram, err := csdf.ParseBytesAllowingDirectives(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: %w", err)
		}

		if err := json.NewEncoder(inout.Stdout).Encode(diagram); err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: writing JSON: %w", err)
		}
		return nil
	}
}
