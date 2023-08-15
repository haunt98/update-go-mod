package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/sourcegraph/conc/pool"
	"github.com/urfave/cli/v2"
)

const maxPoolGoroutine = 8

var reGitHub = regexp.MustCompile(`github\.com/([^/]*)/([^/]*)`)

type GitHubRepoData struct {
	LastCommitAt time.Time
	Name         string
	StarCount    int
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

	p := pool.New().WithMaxGoroutines(maxPoolGoroutine)
	var mMutex sync.RWMutex
	var listMutex sync.Mutex
	for module := range mapImportedModules {
		module := module
		p.Go(func() {
			ctx := context.Background()

			if !reGitHub.MatchString(module) {
				return
			}

			parts := reGitHub.FindStringSubmatch(module)
			if len(parts) != 3 {
				return
			}

			name := parts[0]
			mMutex.RLock()
			if _, ok := mGHRepoData[name]; ok {
				mMutex.RUnlock()
				return
			}
			mMutex.RUnlock()

			mMutex.Lock()
			mGHRepoData[name] = struct{}{}
			mMutex.Unlock()

			owner := parts[1]
			repo := parts[2]

			ghRepo, _, err := a.ghClient.Repositories.Get(ctx, owner, repo)
			if err != nil {
				a.log("GitHub failed to get repo %s/%s: %s\n", owner, repo, err)
			}

			var starCount int
			if ghRepo.StargazersCount != nil {
				starCount = *ghRepo.StargazersCount
			}

			ghCommits, _, err := a.ghClient.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
				ListOptions: github.ListOptions{
					Page:    1,
					PerPage: 1,
				},
			})
			if err != nil {
				a.log("GitHub failed to get commits %s/%s: %s\n", owner, repo, err)
			}

			var lastCommitAt time.Time
			if len(ghCommits) != 0 {
				if ghCommits[0].Commit != nil &&
					ghCommits[0].Commit.Author != nil &&
					ghCommits[0].Commit.Author.Date != nil {
					lastCommitAt = ghCommits[0].Commit.Author.Date.Time
				}
			}

			listMutex.Lock()
			listGHRepoData = append(listGHRepoData, GitHubRepoData{
				LastCommitAt: lastCommitAt,
				Name:         name,
				StarCount:    starCount,
			})
			listMutex.Unlock()
		})
	}
	p.Wait()

	// Sort for consistency
	sort.Slice(listGHRepoData, func(i, j int) bool {
		return listGHRepoData[i].Name < listGHRepoData[j].Name
	})

	// Print
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range listGHRepoData {
		fmt.Fprintf(w, "Module %s\t%d\t⭐\tLast commit %s\n", r.Name, r.StarCount, r.LastCommitAt.Format(time.DateOnly))
	}
	w.Flush()

	return nil
}
