package cli

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"
)

var reGitHub = regexp.MustCompile(`github\.com/([^/]*)/([^/]*)`)

type GitHubRepoData struct {
	UpdatedAt time.Time
	Name      string
	Star      int
}

func (a *action) Overlook(c *cli.Context) error {
	// Optional
	if a.ghClient == nil {
		return nil
	}

	a.getFlags(c)

	mapImportedModules, err := a.runGetImportedModules(c)
	if err != nil {
		return err
	}

	if len(mapImportedModules) == 0 {
		return nil
	}

	listGHRepoData := make([]GitHubRepoData, 0, len(mapImportedModules))
	// To avoid duplicate
	mGHRepoData := make(map[string]struct{})

	for module := range mapImportedModules {
		if !reGitHub.MatchString(module) {
			continue
		}

		parts := reGitHub.FindStringSubmatch(module)
		if len(parts) != 3 {
			continue
		}

		ghRepoName := parts[0]
		if _, ok := mGHRepoData[ghRepoName]; ok {
			continue
		}
		mGHRepoData[ghRepoName] = struct{}{}

		owner := parts[1]
		repo := parts[2]

		ghRepo, _, err := a.ghClient.Repositories.Get(c.Context, owner, repo)
		if err != nil {
			a.log("Failed to get GitHub %s/%s: %s\n", owner, repo, err)
		}

		var ghStar int
		if ghRepo.StargazersCount != nil {
			ghStar = *ghRepo.StargazersCount
		}

		var ghUpdatedAt time.Time
		if ghRepo.UpdatedAt != nil {
			ghUpdatedAt = ghRepo.UpdatedAt.Time
		}

		listGHRepoData = append(listGHRepoData, GitHubRepoData{
			UpdatedAt: ghUpdatedAt,
			Name:      ghRepoName,
			Star:      ghStar,
		})

	}

	// Sort for consistency
	sort.Slice(listGHRepoData, func(i, j int) bool {
		return listGHRepoData[i].Name < listGHRepoData[j].Name
	})

	// Print
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range listGHRepoData {
		fmt.Fprintf(w, "Module %s\t%d\t⭐\tLast updated %s\n", r.Name, r.Star, r.UpdatedAt.Format(time.DateOnly))
	}
	w.Flush()

	return nil
}
