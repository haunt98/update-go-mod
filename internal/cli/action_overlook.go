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

	"github.com/google/go-github/v90/github"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"
)

const maxPoolGoroutine = 8

var reGitHub = regexp.MustCompile(`github\.com/([^/]*)/([^/]*)`)

type GitRepoData struct {
	LastCommitAt   time.Time
	ModulePath     string
	CurrentVersion string
	LatestVersion  string
	StarCount      int
}

func (a *action) Overlook(ctx context.Context, c *cli.Command) error {
	// Optional
	if a.ghClient == nil {
		return nil
	}

	a.getFlags(c)

	mapImportedModules, err := a.runGetImportedModules(ctx)
	if err != nil {
		return err
	}

	if len(mapImportedModules) == 0 {
		return nil
	}

	listGitRepoData := make([]GitRepoData, 0, len(mapImportedModules))
	// To avoid process again
	mProccessedGitRepoData := make(map[string]struct{})

	p := pool.New().WithMaxGoroutines(maxPoolGoroutine)
	var mMutex sync.Mutex
	var listMutex sync.Mutex
	for modulePath, module := range mapImportedModules {
		p.Go(func() {
			ctx := context.WithoutCancel(ctx)

			var latestVersion string
			if module.Update != nil {
				latestVersion = module.Update.Version
			}

			if reGitHub.MatchString(modulePath) {
				ghParts := reGitHub.FindStringSubmatch(modulePath)
				if len(ghParts) != 3 {
					return
				}

				ghRepoName := ghParts[0]
				mMutex.Lock()
				if _, ok := mProccessedGitRepoData[ghRepoName]; ok {
					mMutex.Unlock()
					return
				}
				mProccessedGitRepoData[ghRepoName] = struct{}{}
				mMutex.Unlock()

				ghOwner := ghParts[1]
				ghRepo := ghParts[2]

				ghRepoData, _, err := a.ghClient.Repositories.Get(ctx, ghOwner, ghRepo)
				if err != nil {
					a.log("GitHub failed to get repo %s/%s: %s\n", ghOwner, ghRepo, err)
				}

				var starCount int
				if ghRepoData.StargazersCount != nil {
					starCount = *ghRepoData.StargazersCount
				}

				ghCommits, _, err := a.ghClient.Repositories.ListCommits(ctx, ghOwner, ghRepo, &github.CommitsListOptions{
					ListOptions: github.ListOptions{
						Page:    1,
						PerPage: 1,
					},
				})
				if err != nil {
					a.log("GitHub failed to get commits %s/%s: %s\n", ghOwner, ghRepo, err)
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
				listGitRepoData = append(listGitRepoData, GitRepoData{
					LastCommitAt:   lastCommitAt,
					ModulePath:     modulePath,
					CurrentVersion: module.Version,
					LatestVersion:  latestVersion,
					StarCount:      starCount,
				})
				listMutex.Unlock()

				return
			}

			listMutex.Lock()
			listGitRepoData = append(listGitRepoData, GitRepoData{
				ModulePath:     modulePath,
				CurrentVersion: module.Version,
				LatestVersion:  latestVersion,
			})
			listMutex.Unlock()
		})
	}
	p.Wait()

	// Sort for consistency
	sort.Slice(listGitRepoData, func(i, j int) bool {
		return listGitRepoData[i].ModulePath < listGitRepoData[j].ModulePath
	})

	// Print
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if a.flags.latest {
		fmt.Fprintln(w, "Module\tCurrent\tLatest\t⭐\tLast Commit")
	} else {
		fmt.Fprintln(w, "Module\tCurrent\t⭐\tLast Commit")
	}

	for _, r := range listGitRepoData {
		if a.flags.latest {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ModulePath, r.CurrentVersion, r.LatestVersion, roundK(r.StarCount), r.LastCommitAt.Format(time.DateOnly))
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ModulePath, r.CurrentVersion, roundK(r.StarCount), r.LastCommitAt.Format(time.DateOnly))
	}

	w.Flush()

	return nil
}

// Nearest thounsand
// 1234 -> 1K
func roundK(v int) string {
	if v < 1000 {
		return cast.ToString(v)
	}

	return fmt.Sprintf("%dK", v/1000)
}
