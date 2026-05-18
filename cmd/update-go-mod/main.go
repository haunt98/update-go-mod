package main

import (
	"context"
	"log"
	"strings"

	"github.com/google/go-github/v87/github"

	"github.com/make-go-great/netrc-go"

	"github.com/haunt98/update-go-mod/internal/cli"
)

const (
	netrcPath          = "~/.netrc"
	netrcMachineGitHub = "github.com"
)

func main() {
	app := cli.NewApp(
		initGitHubClient(),
	)
	app.Run(context.Background())
}

func initGitHubClient() *github.Client {
	netrcData, err := netrc.ParseFile(netrcPath)
	if err != nil {
		log.Panicf("netrc: failed to parse file: %v", err)
	}

	var ghAccessToken string
	for _, machine := range netrcData.Machines {
		if machine.Name == netrcMachineGitHub {
			ghAccessToken = machine.Password
			break
		}
	}

	ghAccessToken = strings.TrimSpace(ghAccessToken)
	if ghAccessToken == "" {
		log.Panicf("Empty GitHub access token")
	}

	ghClient, err := github.NewClient(github.WithAuthToken(ghAccessToken))
	if err != nil {
		log.Panicf("github: failed to create client: %v", err)
	}

	return ghClient
}
