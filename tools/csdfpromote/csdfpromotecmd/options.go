package csdfpromotecmd

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// Base is what an !include path is resolved against: the directory the
	// input was read from, or the working directory when it came from stdin.
	Base       string
	NoComments bool
	Werror     bool
	LintOnly   bool
	JSON       bool
	Args       []string
	Bytes      []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfpromote", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfpromote [options] [file.puml|file.png]

Expands the promotion directives of a global Composable State Diagram: every
local diagram named by a <<promote>> block becomes self-loops on the state the
block was written in, carrying the map the family of instances lives in. The
result is plain PlantUML, which every other tool reads.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfpromote path/to/GLOBAL.puml
  $ csdfpromote -lint-only -Werror path/to/GLOBAL.puml
  $ csdfpromote path/to/GLOBAL.puml | csdflivelockfree -
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		base := flags.String("base", "", "directory to resolve !include paths against (default: the directory of the input file)")
		noComments := flags.Bool("no-comments", false, "omit the line comment that says where each generated edge came from")
		werror := flags.Bool("Werror", false, "treat warnings as errors")
		lintOnly := flags.Bool("lint-only", false, "check the directives without printing the expansion")
		asJSON := flags.Bool("json", false, "print the parsed directives as JSON instead of expanding them")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		path, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		resolved := *base
		if resolved == "" {
			resolved = "."
			if path != "" {
				resolved = filepath.Dir(path)
			}
		}

		return &Options{
			Common:     commonOpts,
			Base:       resolved,
			NoComments: *noComments,
			Werror:     *werror,
			LintOnly:   *lintOnly,
			JSON:       *asJSON,
			Args:       args,
			Bytes:      bs,
		}, nil
	}
}
