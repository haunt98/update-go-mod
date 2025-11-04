package cli

import (
	"context"
	"log"

	"github.com/google/go-github/v77/github"
	"github.com/urfave/cli/v3"
)

const (
	defaultDepsFile = ".deps"
)

type action struct {
	ghClient *github.Client
	flags    struct {
		depsFile      string
		depsURL       string
		verbose       bool
		dryRun        bool
		forceIndirect bool
	}
}

func (a *action) RunHelp(ctx context.Context, c *cli.Command) error {
	return cli.ShowAppHelp(c)
}

func (a *action) getFlags(c *cli.Command) {
	a.flags.verbose = c.Bool(flagVerboseName)

	a.flags.depsFile = c.String(flagDepsFileName)
	if a.flags.depsFile == "" {
		a.log("Fallback to default deps file [%s]\n", defaultDepsFile)
		a.flags.depsFile = defaultDepsFile
	}

	a.flags.depsURL = c.String(flagDepsURLName)
	a.flags.dryRun = c.Bool(flagDryRun)
	a.flags.forceIndirect = c.Bool(flagForceIndirectName)

	a.log("Flags %+v\n", a.flags)
}

func (a *action) log(format string, v ...any) {
	if a.flags.verbose {
		log.Printf(format, v...)
	}
}
