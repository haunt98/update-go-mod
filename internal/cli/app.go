package cli

import (
	"os"

	"github.com/make-go-great/color-go"
	"github.com/urfave/cli/v2"
)

const (
	name  = "update-go-mod"
	usage = "excatly like the name says"

	commandRunName  = "run"
	commandRunUsage = "run the program"

	flagVerboseName  = "verbose"
	flagVerboseUsage = "show what is going on"

	flagDepsFileName  = "deps-file"
	flagDepsFileUsage = "file which show what deps need to upgrade"

	flagDepsURLName  = "deps-url"
	flagDepsURLUsage = "url which show what deps need to upgrade"

	flagDryRun     = "dry-run"
	flagDryRunName = "demo what would be done"
)

var aliasFlagVerbose = []string{"v"}

type App struct {
	cliApp *cli.App
}

func NewApp() *App {
	a := &action{}

	cliApp := &cli.App{
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
				},
				Action: a.Run,
			},
		},
		Action: a.RunHelp,
	}

	return &App{
		cliApp: cliApp,
	}
}

func (a *App) Run() {
	if err := a.cliApp.Run(os.Args); err != nil {
		color.PrintAppError(name, err.Error())
	}
}
