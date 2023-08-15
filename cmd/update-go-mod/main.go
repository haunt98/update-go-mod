package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"

	"github.com/make-go-great/netrc-go"

	"github.com/haunt98/update-go-mod/internal/cli"
)

const (
	netrcPath          = "~/.netrc"
	netrcMachineGitHub = "github.com"
)

func main() {
	// Prepare GitHub
	var ghClient *github.Client
	netrcData, err := netrc.ParseFile(netrcPath)
	if err == nil {
		var ghAccessToken string
		for _, machine := range netrcData.Machines {
			if machine.Name == netrcMachineGitHub {
				ghAccessToken = machine.Password
				break
			}
		}

		if ghAccessToken != "" {
			ghTokenSrc := oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: strings.TrimSpace(ghAccessToken),
				},
			)
			ghHTTPClient := oauth2.NewClient(context.Background(), ghTokenSrc)
			ghClient = github.NewClient(ghHTTPClient)
		}
	}

	app := cli.NewApp(ghClient)
	app.Run()
}
