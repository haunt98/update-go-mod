package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/google/go-github/v90/github"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

const maxPoolGoroutine = 8

var (
	reGitHub = regexp.MustCompile(`github\.com/([^/]*)/([^/]*)`)
	reGitLab = regexp.MustCompile(`^(gitlab[^/]+)/(.+)`)
)

type GitRepoData struct {
	LastCommitAt   time.Time
	ModulePath     string
	CurrentVersion string
	LatestVersion  string
	StarCount      int
}

func (a *action) Overlook(ctx context.Context, c *cli.Command) error {
	a.getFlags(c)

	mapImportedModules, err := a.runGetImportedModules(ctx)
	if err != nil {
		return err
	}

	if len(mapImportedModules) == 0 {
		return nil
	}

	listGitRepoData := make([]GitRepoData, 0, len(mapImportedModules))
	var listMutex sync.Mutex

	p := pool.New().WithMaxGoroutines(maxPoolGoroutine)

	for modulePath, module := range mapImportedModules {
		p.Go(func() {
			ctx := context.WithoutCancel(ctx)

			var latestVersion string
			if module.Update != nil {
				latestVersion = module.Update.Version
			}

			gitRepoData := GitRepoData{
				ModulePath:     modulePath,
				CurrentVersion: module.Version,
				LatestVersion:  latestVersion,
			}

			if a.ghClient != nil &&
				reGitHub.MatchString(modulePath) {
				lastCommitAt, starCount, ok := a.getGitHubRepoData(ctx, modulePath)
				if ok {
					gitRepoData.LastCommitAt = lastCommitAt
					gitRepoData.StarCount = starCount
				}
			} else if a.glClients != nil &&
				reGitLab.MatchString(modulePath) {
				lastCommitAt, starCount, ok := a.getGitLabRepoData(ctx, modulePath)
				if ok {
					gitRepoData.LastCommitAt = lastCommitAt
					gitRepoData.StarCount = starCount
				}
			}

			listMutex.Lock()
			listGitRepoData = append(listGitRepoData, gitRepoData)
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

func (a *action) getGitHubRepoData(ctx context.Context, modulePath string) (lastCommitAt time.Time, starCount int, ok bool) {
	ghParts := reGitHub.FindStringSubmatch(modulePath)
	if len(ghParts) != 3 {
		return lastCommitAt, starCount, ok
	}

	ghOwner := ghParts[1]
	ghRepo := ghParts[2]

	ghRepoData, _, err := a.ghClient.Repositories.Get(ctx, ghOwner, ghRepo)
	if err != nil {
		a.log("GitHub failed to get repo %s/%s: %s\n", ghOwner, ghRepo, err)
	}

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
		a.log("GitHub failed to list commits %s/%s: %s\n", ghOwner, ghRepo, err)
	}

	if len(ghCommits) != 0 {
		if ghCommits[0].Commit != nil &&
			ghCommits[0].Commit.Author != nil &&
			ghCommits[0].Commit.Author.Date != nil {
			lastCommitAt = ghCommits[0].Commit.Author.Date.Time
		}
	}

	ok = true
	return lastCommitAt, starCount, ok
}

func (a *action) getGitLabRepoData(ctx context.Context, modulePath string) (lastCommitAt time.Time, starCount int, ok bool) {
	glParts := reGitLab.FindStringSubmatch(modulePath)
	if len(glParts) != 3 {
		return lastCommitAt, starCount, ok
	}

	glHost := glParts[1]

	glClient, existGLClient := a.glClients[glHost]
	if !existGLClient {
		return lastCommitAt, starCount, ok
	}

	// GitLab supports nested groups (group/subgroup/project)
	glProjectPath := glParts[2]
	foundGLRepo := false

	for {
		glProject, _, err := glClient.Projects.GetProject(glProjectPath, nil, gitlab.WithContext(ctx))
		if err != nil {
			a.log("GitLab failed to get project %s: %s\n", glProjectPath, err)

			var glErr *gitlab.ErrorResponse
			if !errors.As(err, &glErr) ||
				glErr.Response == nil ||
				glErr.Response.StatusCode != http.StatusNotFound {
				break
			}

			slashIdx := strings.LastIndex(glProjectPath, "/")
			if slashIdx == -1 {
				break
			}

			glProjectPath = glProjectPath[:slashIdx]
			continue
		}

		starCount = int(glProject.StarCount)
		foundGLRepo = true
		break
	}

	if foundGLRepo {
		glCommits, _, err := glClient.Commits.ListCommits(
			glProjectPath,
			&gitlab.ListCommitsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    1,
					PerPage: 1,
				},
			},
			gitlab.WithContext(ctx),
		)
		if err != nil {
			a.log("GitLab failed to list commits %s: %s\n", glProjectPath, err)
		}

		if len(glCommits) != 0 {
			if glCommits[0].CommittedDate != nil {
				lastCommitAt = *glCommits[0].CommittedDate
			}
		}

		ok = true
	}

	return lastCommitAt, starCount, ok
}

// Nearest thounsand
// 1234 -> 1K
func roundK(v int) string {
	if v < 1000 {
		return cast.ToString(v)
	}

	return fmt.Sprintf("%dK", v/1000)
}
