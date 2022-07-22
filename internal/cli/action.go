package cli

import (
	"log"

	"github.com/urfave/cli/v2"
)

type action struct {
	flags struct {
		depsFile string
		depsURL  string
		verbose  bool
		dryRun   bool
	}
}

func (a *action) RunHelp(c *cli.Context) error {
	return cli.ShowAppHelp(c)
}

func (a *action) getFlags(c *cli.Context) {
	a.flags.verbose = c.Bool(flagVerbose)
	a.flags.depsFile = c.String(flagDepsFile)
	a.flags.depsURL = c.String(flagDepsURL)
	a.flags.dryRun = c.Bool(flagDryRun)

	a.log("flags %+v", a.flags)
}

func (a *action) log(format string, v ...interface{}) {
	if a.flags.verbose {
		log.Printf(format, v...)
	}
}
