package main

import (
	"context"
	"log"
	"strings"

	"github.com/google/go-github/v88/github"

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
		log.Fatalf("netrc: failed to parse file: %v\n", err)
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
		log.Fatalln("Empty GitHub access token")
	}

	ghClient, err := github.NewClient(github.WithAuthToken(ghAccessToken))
	if err != nil {
		log.Fatalf("github: failed to create client: %v\n", err)
	}

	return ghClient
}
