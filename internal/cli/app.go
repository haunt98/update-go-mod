package cli

import (
	"context"
	"os"

	"github.com/google/go-github/v73/github"
	"github.com/urfave/cli/v3"

	"github.com/make-go-great/color-go"
)

const (
	name  = "update-go-mod"
	usage = "excatly like the name says"

	commandRunName  = "run"
	commandRunUsage = "run the program"

	commandOverlookName  = "overlook"
	commandOverlookUsage = "take a quick lock"

	flagVerboseName  = "verbose"
	flagVerboseUsage = "show what is going on"

	flagDepsFileName  = "deps-file"
	flagDepsFileUsage = "file which show what deps need to upgrade"

	flagDepsURLName  = "deps-url"
	flagDepsURLUsage = "url which show what deps need to upgrade"

	flagDryRun     = "dry-run"
	flagDryRunName = "demo what would be done"

	flagForceIndirectName  = "force-indirect"
	flagForceIndirectUsage = "force update indirect module"
)

var aliasFlagVerbose = []string{"v"}

type App struct {
	cliApp *cli.Command
}

func NewApp(
	ghClient *github.Client,
) *App {
	a := &action{
		ghClient: ghClient,
	}

	cliApp := &cli.Command{
		Name:  name,
		Usage: usage,
		Commands: []*cli.Command{
			{
				Name:  commandRunName,
				Usage: commandRunUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    flagVerboseName,
						Aliases: aliasFlagVerbose,
						Usage:   flagVerboseUsage,
					},
					&cli.StringFlag{
						Name:  flagDepsFileName,
						Usage: flagDepsFileUsage,
					},
					&cli.StringFlag{
						Name:  flagDepsURLName,
						Usage: flagDepsURLUsage,
					},
					&cli.BoolFlag{
						Name:  flagDryRun,
						Usage: flagDryRunName,
					},
					&cli.BoolFlag{
						Name:  flagForceIndirectName,
						Usage: flagForceIndirectUsage,
					},
				},
				Action: a.Run,
			},
			{
				Name:  commandOverlookName,
				Usage: commandOverlookUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    flagVerboseName,
						Aliases: aliasFlagVerbose,
						Usage:   flagVerboseUsage,
					},
				},
				Action: a.Overlook,
			},
		},
		Action: a.RunHelp,
	}

	return &App{
		cliApp: cliApp,
	}
}

func (a *App) Run(ctx context.Context) {
	if err := a.cliApp.Run(ctx, os.Args); err != nil {
		color.PrintAppError(name, err.Error())
	}
}
