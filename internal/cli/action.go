package cli

import (
	"log"

	"github.com/urfave/cli/v2"
)

const (
	defaultDepsFile = ".deps"
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
	a.flags.verbose = c.Bool(flagVerboseName)

	a.flags.depsFile = c.String(flagDepsFileName)
	if a.flags.depsFile == "" {
		a.log("Fallback to default deps file [%s]\n", defaultDepsFile)
		a.flags.depsFile = defaultDepsFile
	}

	a.flags.depsURL = c.String(flagDepsURLName)
	a.flags.dryRun = c.Bool(flagDryRun)

	a.log("Flags %+v", a.flags)
}

func (a *action) log(format string, v ...interface{}) {
	if a.flags.verbose {
		log.Printf(format, v...)
	}
}
