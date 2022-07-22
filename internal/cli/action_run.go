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
	gitDirectory    = ".git"
	vendorDirectory = "vendor"
	goModFile       = "go.mod"
	goSumFile       = "go.sum"
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

	mapImportedModules, err := a.runGetImportedModules(c)
	if err != nil {
		return err
	}

	// Check vendor exist to run go mod vendor
	existVendor := false
	if _, err := os.Stat(vendorDirectory); err == nil {
		existVendor = true
	}

	depsStr, err := a.runReadDepsFile(c)
	if err != nil {
		return err
	}

	if depsStr == "" {
		return nil
	}

	// Read deps file line by line to upgrade
	successUpgradedModules := make([]Module, 0, 100)
	modulePaths := strings.Split(depsStr, "\n")
	for _, modulePath := range modulePaths {
		successUpgradedModules, err = a.runUpgradeModule(c, mapImportedModules, successUpgradedModules, modulePath)
		if err != nil {
			color.PrintAppError(name, err.Error())
		}
	}

	if err := a.runGoMod(c, existVendor); err != nil {
		return err
	}

	if err := a.runGitCommit(c, successUpgradedModules, existVendor); err != nil {
		return err
	}

	return nil
}

func (a *action) runGetImportedModules(c *cli.Context) (map[string]struct{}, error) {
	// Get all imported modules
	goListAllArgs := []string{"list", "-m", "-json", "all"}
	goOutput, err := exec.CommandContext(c.Context, "go", goListAllArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run go %+v: %w", strings.Join(goListAllArgs, " "), err)
	}

	// goAllOutput is like {...}\n{...}\n{...}
	// Missing [] and , for json
	goOutputStr := strings.ReplaceAll(strings.TrimSpace(string(goOutput)), "\n", "")
	goOutputStr = strings.ReplaceAll(goOutputStr, "}{", "},{")
	goOutputStr = "[" + goOutputStr + "]"

	importedModules := make([]Module, 0, 100)
	if err := json.Unmarshal([]byte(goOutputStr), &importedModules); err != nil {
		return nil, fmt.Errorf("failed to json unmarshal: %w", err)
	}
	a.log("go output: %s", string(goOutput))

	mapImportedModules := make(map[string]struct{})
	for _, importedModule := range importedModules {
		mapImportedModules[importedModule.Path] = struct{}{}
	}
	a.log("imported modules: %+v\n", importedModules)

	return mapImportedModules, nil
}

func (a *action) runReadDepsFile(c *cli.Context) (string, error) {
	// Try to read from url first
	if a.flags.depsURL != "" {
		depsURL, err := url.Parse(a.flags.depsURL)
		if err != nil {
			return "", fmt.Errorf("failed to parse deps file url %s: %w", a.flags.depsURL, err)
		}

		httpRsp, err := http.Get(depsURL.String())
		if err != nil {
			return "", fmt.Errorf("failed to http get %s: %w", depsURL.String(), err)
		}
		defer httpRsp.Body.Close()

		if httpRsp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("http status code not ok %d: %w", httpRsp.StatusCode, ErrFailedStatusCode)
		}

		depsBytes, err := io.ReadAll(httpRsp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read http response body: %w", err)
		}

		return strings.TrimSpace(string(depsBytes)), nil
	}

	// If empty url, try to read from file
	depsBytes, err := os.ReadFile(a.flags.depsFile)
	if err != nil {
		if os.IsNotExist(err) {
			color.PrintAppWarning(name, fmt.Sprintf("deps file [%s] not found", a.flags.depsFile))
			return "", nil
		}

		return "", fmt.Errorf("failed to read file %s: %w", a.flags.depsFile, err)
	}

	return strings.TrimSpace(string(depsBytes)), nil
}

func (a *action) runUpgradeModule(
	c *cli.Context,
	mapImportedModules map[string]struct{},
	successUpgradedModules []Module,
	modulePath string,
) ([]Module, error) {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return successUpgradedModules, nil
	}
	a.log("line: %s", modulePath)

	// Check if modulePath is imported module, otherwise skip
	if _, ok := mapImportedModules[modulePath]; !ok {
		a.log("%s is not imported module", modulePath)
		return successUpgradedModules, nil
	}

	// Get module latest version
	goListArgs := []string{"list", "-m", "-u", "-json", modulePath}
	goOutput, err := exec.CommandContext(c.Context, "go", goListArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("failed to run go %+v: %w", strings.Join(goListArgs, " "), err)
	}
	a.log("go output: %s", string(goOutput))

	module := Module{}
	if err := json.Unmarshal(goOutput, &module); err != nil {
		return successUpgradedModules, fmt.Errorf("failed to json unmarshal: %w", err)
	}
	a.log("module: %+v", module)

	if module.Update == nil {
		color.PrintAppOK(name, fmt.Sprintf("You already have latest [%s] version [%s]", module.Path, module.Version))
		return successUpgradedModules, nil
	}

	// Upgrade module
	if a.flags.dryRun {
		color.PrintAppOK(name, fmt.Sprintf("Will upgrade [%s] version [%s] to [%s]", module.Path, module.Version, module.Update.Version))
		return successUpgradedModules, nil
	}

	goGetArgs := []string{"get", "-d", modulePath + "@" + module.Update.Version}
	goOutput, err = exec.CommandContext(c.Context, "go", goGetArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("failed to run go %+v: %w", strings.Join(goGetArgs, " "), err)
	}
	a.log("go output: %s", string(goOutput))

	successUpgradedModules = append(successUpgradedModules, module)

	color.PrintAppOK(name, fmt.Sprintf("Upgraded [%s] version [%s] to [%s] success", module.Path, module.Version, module.Update.Version))

	return successUpgradedModules, nil
}

func (a *action) runGoMod(c *cli.Context, existVendor bool) error {
	if a.flags.dryRun {
		return nil
	}

	// go mod tidy
	goModArgs := []string{"mod", "tidy"}
	goOutput, err := exec.CommandContext(c.Context, "go", goModArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
	}
	a.log("go output: %s", string(goOutput))

	if existVendor {
		// go mod vendor
		goModArgs = []string{"mod", "vendor"}
		goOutput, err = exec.CommandContext(c.Context, "go", goModArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
		}
		a.log("go output: %s", string(goOutput))
	}

	return nil
}

func (a *action) runGitCommit(c *cli.Context, successUpgradedModules []Module, existVendor bool) error {
	if a.flags.dryRun {
		return nil
	}

	// If not exist git, stop
	if _, err := os.Stat(gitDirectory); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to stat %s: %w", gitDirectory, err)
	}

	// If there is no upgrade module, stop
	if len(successUpgradedModules) == 0 {
		return nil
	}

	// git add
	gitAddArgs := []string{"add", goModFile, goSumFile}
	if existVendor {
		gitAddArgs = append(gitAddArgs, vendorDirectory)
	}
	gitOutput, err := exec.CommandContext(c.Context, "git", gitAddArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git %+v: %w", strings.Join(gitAddArgs, " "), err)
	}
	a.log("git output: %s", string(gitOutput))

	// git commit
	gitCommitMessage := "build: upgrade modules\n"
	for _, module := range successUpgradedModules {
		gitCommitMessage += fmt.Sprintf("\n%s: %s -> %s", module.Path, module.Version, module.Update.Version)
	}
	gitCommitArgs := []string{"commit", "-m", `"` + gitCommitMessage + `"`}
	gitOutput, err = exec.CommandContext(c.Context, "git", gitCommitArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git %+v: %w", strings.Join(gitCommitArgs, " "), err)
	}
	a.log("git output: %s", string(gitOutput))

	return nil
}
