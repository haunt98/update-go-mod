package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/make-go-great/color-go"
	"github.com/urfave/cli/v2"
)

const (
	gitDirectory = ".git"
)

var (
	ErrInvalidModuleVersion = errors.New("invalid module version")
	ErrFailedStatusCode     = errors.New("failed status code")
)

// See https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules
type Module struct {
	Update  *Module
	Path    string
	Version string
}

func (a *action) Run(c *cli.Context) error {
	a.getFlags(c)

	// Get all imported modules
	goListAllArgs := []string{"list", "-m", "-json", "all"}
	goOutput, err := exec.CommandContext(c.Context, "go", goListAllArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go %+v: %w", strings.Join(goListAllArgs, " "), err)
	}

	// goAllOutput is like {...}\n{...}\n{...}
	// Missing [] and , for json
	goOutputStr := strings.ReplaceAll(strings.TrimSpace(string(goOutput)), "\n", "")
	goOutputStr = strings.ReplaceAll(goOutputStr, "}{", "},{")
	goOutputStr = "[" + goOutputStr + "]"

	importedModules := make([]Module, 0, 100)
	if err := json.Unmarshal([]byte(goOutputStr), &importedModules); err != nil {
		return fmt.Errorf("failed to json unmarshal: %w", err)
	}
	a.log("go output: %s", string(goOutput))

	mapImportedModules := make(map[string]struct{})
	for _, importedModule := range importedModules {
		mapImportedModules[importedModule.Path] = struct{}{}
	}
	a.log("imported modules: %+v\n", importedModules)

	// Try to parse url first
	var depsFileStr string
	depsFileURL, err := url.Parse(a.flags.depsFile)
	if err == nil {
		httpRsp, err := http.Get(depsFileURL.String())
		if err != nil {
			return fmt.Errorf("failed to http get %s: %w", depsFileURL.String(), err)
		}
		defer httpRsp.Body.Close()

		if httpRsp.StatusCode != http.StatusOK {
			return fmt.Errorf("http status code not ok %d: %w", httpRsp.StatusCode, ErrFailedStatusCode)
		}

		depsFileBytes, err := io.ReadAll(httpRsp.Body)
		if err != nil {
			return fmt.Errorf("failed to read http response body: %w", err)
		}

		depsFileStr = string(depsFileBytes)
	} else {
		a.log("url parse error: %s", err)

		// If not url, try to read local file
		depsFileBytes, err := os.ReadFile(a.flags.depsFile)
		if err != nil {
			if os.IsNotExist(err) {
				color.PrintAppWarning(name, fmt.Sprintf("deps file [%s] not found", a.flags.depsFile))
				return nil
			}

			return fmt.Errorf("failed to read file %s: %w", a.flags.depsFile, err)
		}

		depsFileStr = strings.TrimSpace(string(depsFileBytes))
	}

	// Read deps file line by line to upgrade
	successUpgradeModules := make([]Module, 0, 100)

	lines := strings.Split(depsFileStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		a.log("line: %s", line)

		// Check if line is imported module, otherwise skip
		if _, ok := mapImportedModules[line]; !ok {
			a.log("%s is not imported module", line)
			continue
		}

		// Get module latest version
		goListArgs := []string{"list", "-m", "-u", "-json", line}
		goOutput, err = exec.CommandContext(c.Context, "go", goListArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run go %+v: %w", strings.Join(goListArgs, " "), err)
		}
		a.log("go output: %s", string(goOutput))

		module := Module{}
		if err := json.Unmarshal(goOutput, &module); err != nil {
			return fmt.Errorf("failed to json unmarshal: %w", err)
		}
		a.log("module: %+v", module)

		if module.Update == nil {
			color.PrintAppOK(name, fmt.Sprintf("You already have latest [%s] version [%s]", module.Path, module.Version))
			continue
		}

		// Upgrade module
		if a.flags.dryRun {
			color.PrintAppOK(name, fmt.Sprintf("Will upgrade [%s] version [%s] to [%s]", module.Path, module.Version, module.Update.Version))
			continue
		}

		goGetArgs := []string{"get", "-d", line + "@" + module.Update.Version}
		goOutput, err = exec.CommandContext(c.Context, "go", goGetArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run go %+v: %w", strings.Join(goGetArgs, " "), err)
		}
		a.log("go output: %s", string(goOutput))

		successUpgradeModules = append(successUpgradeModules, module)

		color.PrintAppOK(name, fmt.Sprintf("Upgraded [%s] version [%s] to [%s] success", module.Path, module.Version, module.Update.Version))
	}

	// go mod tidy
	goModArgs := []string{"mod", "tidy"}
	goOutput, err = exec.CommandContext(c.Context, "go", goModArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
	}
	a.log("go output: %s", string(goOutput))

	// If exist git, auto commit
	if _, err := os.Stat(gitDirectory); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to stat %s: %w", gitDirectory, err)
	}

	if a.flags.dryRun {
		return nil
	}

	if len(successUpgradeModules) == 0 {
		return nil
	}

	gitCommitMessage := "build: upgrade modules\n"
	for _, module := range successUpgradeModules {
		gitCommitMessage += fmt.Sprintf("\n%s: %s -> %s", module.Path, module.Version, module.Update.Version)
	}
	gitCommitArgs := []string{"commit", "-m", `"` + gitCommitMessage + `"`}
	gitOutput, err := exec.CommandContext(c.Context, "git", gitCommitArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git %+v: %w", strings.Join(gitCommitArgs, " "), err)
	}
	a.log("git output: %s", string(gitOutput))

	return nil
}
