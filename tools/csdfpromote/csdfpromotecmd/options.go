package csdfpromotecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// BaseDir is the directory the paths of promote directives are resolved against.
	BaseDir string
	// NoComments suppresses the line comment that records where an expanded
	// edge came from.
	NoComments bool
	// Werror makes a warning as fatal as an error.
	Werror bool
	// TemplatePath names a file of clause templates that replaces the
	// generated phrases.
	TemplatePath string
	// LintOnly checks the directives without expanding them.
	LintOnly bool
	Bytes    []byte
	// Args is the raw command line, kept to record the command that generated
	// the expanded diagram.
	Args []string
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

Expands the promotion directives of a Composable State Diagram (promote, sync
and constrain) into ordinary edges, and prints the result as PlantUML. Every
other tool refuses a diagram that still carries directives, so this is the
first step of any pipeline over a promoted specification.
A file argument, a "-" argument, and standard input are all equivalent.

Relative paths of promote directives are resolved against the directory of the
input file, or against the current directory when the input is read from
standard input. Use -base to override.

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
		baseFlag := flags.String("base", "", `directory the paths of promote directives are resolved against (default: the directory of the input file, or "." for standard input)`)
		noCommentsFlag := flags.Bool("no-comments", false, "do not record where each expanded edge came from")
		werrorFlag := flags.Bool("Werror", false, "treat warnings as errors")
		lintOnlyFlag := flags.Bool("lint-only", false, "check the directives without expanding them")
		templateFlag := flags.String("template", "", "file of clause templates that replaces the generated phrases (default: the symbolic ones)")

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

		inputPath, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		baseDir := *baseFlag
		if baseDir == "" {
			baseDir = csdf.BaseDirOf(inputPath)
		}

		return &Options{
			Common:       commonOpts,
			BaseDir:      baseDir,
			NoComments:   *noCommentsFlag,
			Werror:       *werrorFlag,
			LintOnly:     *lintOnlyFlag,
			TemplatePath: *templateFlag,
			Bytes:        bs,
			Args:         args,
		}, nil
	}
}
