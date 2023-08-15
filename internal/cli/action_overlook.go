package cli

import (
	"fmt"
	"regexp"

	"github.com/urfave/cli/v2"
)

var reGitHub = regexp.MustCompile(`github\.com/([^/]*)/([^/]*)`)

func (a *action) Overlook(c *cli.Context) error {
	a.getFlags(c)

	mapImportedModules, err := a.runGetImportedModules(c)
	if err != nil {
		return err
	}

	for module := range mapImportedModules {
		if !reGitHub.MatchString(module) {
			continue
		}

		parts := reGitHub.FindStringSubmatch(module)
		if len(parts) != 3 {
			continue
		}

		owner := parts[1]
		repo := parts[2]

		ghRepo, _, err := a.ghClient.Repositories.Get(c.Context, owner, repo)
		if err != nil {
			a.log("Failed to get GitHub %s/%s: %s\n", owner, repo, err)
		}

		starCount := 0
		if ghRepo.StargazersCount != nil {
			starCount = *ghRepo.StargazersCount
		}
		fmt.Printf("Module %s: Star %d\n", module, starCount)
	}

	return nil
}
