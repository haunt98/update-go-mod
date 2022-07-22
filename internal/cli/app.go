package cli

import (
	"os"

	"github.com/make-go-great/color-go"
	"github.com/urfave/cli/v2"
)

const (
	name  = "update-go-mod"
	usage = "excatly like the name says"

	flagVerbose  = "verbose"
	flagDepsFile = "deps-file"
	flagDryRun   = "dry-run"

	commandRun = "run"

	usageCommandRun  = "run the program"
	usageFlagVerbose = "show what is going on"
	usageDepsFile    = "show what deps need to upgrade"
	usageDryRun      = "demo what would be done"
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
				Name:  commandRun,
				Usage: usageCommandRun,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    flagVerbose,
						Aliases: aliasFlagVerbose,
						Usage:   usageFlagVerbose,
					},
					&cli.StringFlag{
						Name:     flagDepsFile,
						Usage:    usageDepsFile,
						Required: true,
					},
					&cli.BoolFlag{
						Name:  flagDryRun,
						Usage: usageDryRun,
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
