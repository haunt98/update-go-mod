package cli

import (
	"context"
	"log"

	"github.com/google/go-github/v90/github"
	"github.com/urfave/cli/v3"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

const (
	defaultDepsFile = ".deps"
)

type action struct {
	ghClient  *github.Client
	glClients map[string]*gitlab.Client
	flags     struct {
		depsFile string
		verbose  bool
		dryRun   bool
		latest   bool
		extra    bool
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

	a.flags.dryRun = c.Bool(flagDryRunName)
	a.flags.latest = c.Bool(flagLatestName)
	a.flags.extra = c.Bool(flagExtraName)

	a.log("Flags %+v\n", a.flags)
}

func (a *action) log(format string, v ...any) {
	if a.flags.verbose {
		log.Printf(format, v...)
	}
}
