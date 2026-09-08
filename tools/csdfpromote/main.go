package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfpromote/csdfpromotecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfpromotecmd.NewParseOptionsFunc(),
		csdfpromotecmd.NewMainFunc(),
	).Run()
}
