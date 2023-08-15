package main

import (
	"context"
	"log"
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
	netrcData, err := netrc.ParseFile(netrcPath)
	if err != nil {
		log.Fatalln("Failed to read file netrc", err)
	}

	var ghAccessToken string
	for _, machine := range netrcData.Machines {
		if machine.Name == netrcMachineGitHub {
			ghAccessToken = machine.Password
			break
		}
	}
	if ghAccessToken == "" {
		log.Fatalln("Empty GitHub token in netrc")
	}

	ghTokenSrc := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: strings.TrimSpace(ghAccessToken),
		},
	)
	ghHTTPClient := oauth2.NewClient(context.Background(), ghTokenSrc)
	ghClient := github.NewClient(ghHTTPClient)

	app := cli.NewApp(ghClient)
	app.Run()
}
