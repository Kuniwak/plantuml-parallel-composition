package csdfhelpcmd

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfcomp/csdfcompcmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfevents/csdfeventscmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfhide/csdfhidecmd"
	"github.com/Kuniwak/puml-parallel/tools/csdflivelockfree/csdflivelockfreecmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfnorm/csdfnormcmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfparallel/csdfparallelcmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfparse/csdfparsecmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfpromote/csdfpromotecmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfrefinement/csdfrefinementcmd"
	replcmd "github.com/Kuniwak/puml-parallel/tools/csdfrepl/csdfreplcmd"
	clientcmd "github.com/Kuniwak/puml-parallel/tools/csdfreplcmd/csdfreplcmdcmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfrepld/csdfrepldcmd"
	"github.com/Kuniwak/puml-parallel/tools/csdfsort/csdfsortcmd"
	"github.com/Kuniwak/puml-parallel/tools/obligationirc/obligationirccmd"
	"github.com/Kuniwak/puml-parallel/tools/toolsdoc"
)

// Registry returns every tool csdfhelp bundles, ordered as the README
// introduces them. It is a function rather than a variable so that csdfhelp's
// own entry, whose command function lives in this package, can be built
// without an initialization cycle, and so that no caller can mutate the
// catalog.
func Registry() []toolsdoc.Entry {
	return []toolsdoc.Entry{
		{
			Name:    "csdfparse",
			Summary: "Parses a Composable State Diagram and prints the parsed structure as JSON.",
			Run:     tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()),
		},
		{
			Name:    "csdfevents",
			Summary: "Prints the events used across one or more Composable State Diagrams.",
			Run:     tools.NewCommandFunc(csdfeventscmd.NewParseOptionsFunc(), csdfeventscmd.NewMainFunc()),
		},
		{
			Name:    "csdfpromote",
			Summary: "Expands the promotion directives of a Composable State Diagram into ordinary edges.",
			Run:     tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc()),
		},
		{
			Name:    "csdfparallel",
			Summary: "Composes Composable State Diagrams in parallel following CSP interface parallel semantics.",
			Run:     tools.NewCommandFunc(csdfparallelcmd.NewParseOptionsFunc(), csdfparallelcmd.NewMainFunc()),
		},
		{
			Name:    "csdfhide",
			Summary: "Hides events of a Composable State Diagram following CSP hiding semantics.",
			Run:     tools.NewCommandFunc(csdfhidecmd.NewParseOptionsFunc(), csdfhidecmd.NewMainFunc()),
		},
		{
			Name:    "csdfcomp",
			Summary: "Composes the Composable State Diagrams referred to by a composition tree.",
			Run:     tools.NewCommandFunc(csdfcompcmd.NewParseOptionsFunc(), csdfcompcmd.NewMainFunc()),
		},
		{
			Name:    "csdfnorm",
			Summary: "Normalizes a Composable State Diagram via subset construction with tau-closure.",
			Run:     tools.NewCommandFunc(csdfnormcmd.NewParseOptionsFunc(), csdfnormcmd.NewMainFunc()),
		},
		{
			Name:    "csdfsort",
			Summary: "Prints a Composable State Diagram in canonical order without changing its meaning.",
			Run:     tools.NewCommandFunc(csdfsortcmd.NewParseOptionsFunc(), csdfsortcmd.NewMainFunc()),
		},
		{
			Name:    "csdflivelockfree",
			Summary: "Compiles a livelock-freedom proof obligation as JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(csdflivelockfreecmd.NewParseOptionsFunc(), csdflivelockfreecmd.NewMainFunc()),
		},
		{
			Name:    "csdfrefinement",
			Summary: "Compiles a refinement proof obligation for two Composable State Diagrams as JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(csdfrefinementcmd.NewParseOptionsFunc(), csdfrefinementcmd.NewMainFunc()),
		},
		{
			Name:    "obligationirc",
			Summary: "Compiles the livelock-freedom proof-obligation IR to JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(obligationirccmd.NewParseOptionsFunc(), obligationirccmd.NewMainFunc()),
		},
		{
			Name:    "csdfrepl",
			Summary: "Interactively explores a Composable State Diagram.",
			Run:     tools.NewCommandFunc(replcmd.NewParseOptionsFunc(), replcmd.NewMainFunc()),
		},
		{
			Name:    "csdfrepld",
			Summary: "Runs the CSDF REPL daemon, holding exploration sessions behind a Unix domain socket.",
			Run:     tools.NewCommandFunc(csdfrepldcmd.NewParseOptionsFunc(), csdfrepldcmd.NewMainFunc()),
		},
		{
			Name:    "csdfreplcmd",
			Summary: "One-shot client for the csdfrepld daemon, for headless exploration by agents.",
			Run:     tools.NewSubcommandFunc("csdfreplcmd", clientcmd.Description, clientcmd.Subcommands()),
			Subs:    clientcmd.Subcommands(),
		},
		{
			Name:    "csdfhelp",
			Summary: "Prints the help of every tool in this repository as one Markdown document.",
			Hidden:  true,
			Run:     NewCommandFunc(),
		},
	}
}
