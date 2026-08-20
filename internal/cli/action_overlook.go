package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

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
	LastActivityAt time.Time
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
			if a.flags.latest &&
				module.Update != nil {
				latestVersion = module.Update.Version
			}

			gitRepoData := GitRepoData{
				ModulePath:     modulePath,
				CurrentVersion: module.Version,
				LatestVersion:  latestVersion,
			}

			if a.flags.extra {
				if a.ghClient != nil &&
					reGitHub.MatchString(modulePath) {
					lastActivityAt, starCount, ok := a.getGitHubRepoData(ctx, modulePath)
					if ok {
						gitRepoData.LastActivityAt = lastActivityAt
						gitRepoData.StarCount = starCount
					}
				} else if a.glClients != nil &&
					reGitLab.MatchString(modulePath) {
					lastActivityAt, starCount, ok := a.getGitLabRepoData(ctx, modulePath)
					if ok {
						gitRepoData.LastActivityAt = lastActivityAt
						gitRepoData.StarCount = starCount
					}
				}
			}

			listMutex.Lock()
			listGitRepoData = append(listGitRepoData, gitRepoData)
			listMutex.Unlock()
		})
	}

	p.Wait()

	// Sort for consistency
	slices.SortFunc(listGitRepoData, func(a, b GitRepoData) int {
		return cmp.Compare(a.ModulePath, b.ModulePath)
	})

	// Print
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if a.flags.latest {
		fmt.Fprintln(w, "Module\tCurrent\tLatest\tStar\tLast activity")
	} else {
		fmt.Fprintln(w, "Module\tCurrent\tStar\tLast activity")
	}

	for _, r := range listGitRepoData {
		var lastActivityAtStr string
		if !r.LastActivityAt.IsZero() {
			lastActivityAtStr = r.LastActivityAt.Format(time.DateOnly)
		}

		if a.flags.latest {
			fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\t%s\n",
				r.ModulePath,
				r.CurrentVersion,
				r.LatestVersion,
				roundK(r.StarCount),
				lastActivityAtStr,
			)
		} else {
			fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\n",
				r.ModulePath,
				r.CurrentVersion,
				roundK(r.StarCount),
				lastActivityAtStr,
			)
		}
	}

	w.Flush()

	return nil
}

func (a *action) getGitHubRepoData(ctx context.Context, modulePath string) (lastActivityAt time.Time, starCount int, ok bool) {
	ghParts := reGitHub.FindStringSubmatch(modulePath)
	if len(ghParts) != 3 {
		return lastActivityAt, starCount, ok
	}

	ghOwner := ghParts[1]
	ghRepo := ghParts[2]

	ghRepoData, _, err := a.ghClient.Repositories.Get(ctx, ghOwner, ghRepo)
	if err != nil {
		a.log("GitHub failed to get repo %s/%s: %s\n", ghOwner, ghRepo, err)
		return lastActivityAt, starCount, ok
	}

	if ghRepoData.PushedAt != nil {
		lastActivityAt = ghRepoData.PushedAt.Time
	}

	if ghRepoData.StargazersCount != nil {
		starCount = *ghRepoData.StargazersCount
	}

	ok = true

	return lastActivityAt, starCount, ok
}

func (a *action) getGitLabRepoData(ctx context.Context, modulePath string) (lastActivityAt time.Time, starCount int, ok bool) {
	glParts := reGitLab.FindStringSubmatch(modulePath)
	if len(glParts) != 3 {
		return lastActivityAt, starCount, ok
	}

	glHost := glParts[1]

	glClient, existGLClient := a.glClients[glHost]
	if !existGLClient {
		return lastActivityAt, starCount, ok
	}

	// GitLab supports nested groups (group/subgroup/project)
	glProjectPath := glParts[2]

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

		if glProject.LastActivityAt != nil {
			lastActivityAt = *glProject.LastActivityAt
		}

		starCount = int(glProject.StarCount)

		ok = true

		break
	}

	return lastActivityAt, starCount, ok
}

// Nearest thounsand
// 1234 -> 1K
func roundK(v int) string {
	if v <= 0 {
		return ""
	}

	if v < 1000 {
		return cast.ToString(v)
	}

	return fmt.Sprintf("%dK", v/1000)
}
