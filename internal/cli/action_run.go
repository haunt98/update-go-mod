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

	"github.com/urfave/cli/v2"

	"github.com/make-go-great/color-go"
)

const (
	gitDirectory    = ".git"
	vendorDirectory = "vendor"

	goModFile = "go.mod"
	goSumFile = "go.sum"

	defaultCountModule = 100
	depsFileComment    = "#"
)

var (
	ErrInvalidModuleVersion = errors.New("invalid module version")
	ErrFailedStatusCode     = errors.New("failed status code")
)

// See https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules
type Module struct {
	Update   *Module
	Replace  *Module
	Path     string
	Version  string
	Main     bool
	Indirect bool
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

	depsStr, useDepFile, err := a.runReadDepsFile()
	if err != nil {
		return err
	}

	if depsStr == "" {
		return nil
	}

	// Read deps file line by line to upgrade
	successUpgradedModules := make([]Module, 0, defaultCountModule)
	modulePaths := strings.Split(depsStr, "\n")
	for _, modulePath := range modulePaths {
		successUpgradedModules, err = a.runUpgradeModule(
			c,
			mapImportedModules,
			successUpgradedModules,
			modulePath,
		)
		if err != nil {
			color.PrintAppError(name, err.Error())
		}
	}

	if err := a.runGoMod(c, existVendor); err != nil {
		return err
	}

	if err := a.runGitCommit(c, successUpgradedModules, existVendor, useDepFile); err != nil {
		return err
	}

	return nil
}

// Get all imported modules
func (a *action) runGetImportedModules(c *cli.Context) (map[string]Module, error) {
	goListAllArgs := []string{"list", "-m", "-json", "all"}
	goOutput, err := exec.CommandContext(c.Context, "go", goListAllArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run go %s: %w", strings.Join(goListAllArgs, " "), err)
	}

	// goAllOutput is like {...}\n{...}\n{...}
	// Missing [] and , for json
	goOutputStr := strings.ReplaceAll(strings.TrimSpace(string(goOutput)), "\n", "")
	goOutputStr = strings.ReplaceAll(goOutputStr, "}{", "},{")
	goOutputStr = "[" + goOutputStr + "]"

	importedModules := make([]Module, 0, defaultCountModule)
	if err := json.Unmarshal([]byte(goOutputStr), &importedModules); err != nil {
		return nil, fmt.Errorf("failed to json unmarshal: %w", err)
	}
	a.log("Go output: %s\n", string(goOutput))

	mapImportedModules := make(map[string]Module)
	for _, importedModule := range importedModules {
		// Ignore main module
		if importedModule.Main {
			continue
		}

		// Ignore indirect module
		if importedModule.Indirect && !a.flags.forceIndirect {
			continue
		}

		// Ignore replace module
		if importedModule.Replace != nil {
			continue
		}

		mapImportedModules[importedModule.Path] = importedModule
	}

	a.log("Imported modules: %+v\n", importedModules)

	return mapImportedModules, nil
}

func (a *action) runReadDepsFile() (depsStr string, useDepFile bool, err error) {
	// Try to read from url first
	if a.flags.depsURL != "" {
		depsURL, err := url.Parse(a.flags.depsURL)
		if err != nil {
			return "", false, fmt.Errorf("failed to parse deps file url %s: %w", a.flags.depsURL, err)
		}

		// nolint:noctx
		httpRsp, err := http.Get(depsURL.String())
		if err != nil {
			return "", false, fmt.Errorf("failed to http get %s: %w", depsURL.String(), err)
		}
		defer httpRsp.Body.Close()

		if httpRsp.StatusCode != http.StatusOK {
			return "", false, fmt.Errorf("http status code not ok %d: %w", httpRsp.StatusCode, ErrFailedStatusCode)
		}

		depsBytes, err := io.ReadAll(httpRsp.Body)
		if err != nil {
			return "", false, fmt.Errorf("failed to read http response body: %w", err)
		}

		return strings.TrimSpace(string(depsBytes)), false, nil
	}

	// If empty url, try to read from file
	depsBytes, err := os.ReadFile(a.flags.depsFile)
	if err != nil {
		if os.IsNotExist(err) {
			color.PrintAppWarning(name, fmt.Sprintf("deps file [%s] not found", a.flags.depsFile))
			return "", false, nil
		}

		return "", false, fmt.Errorf("failed to read file %s: %w", a.flags.depsFile, err)
	}

	return strings.TrimSpace(string(depsBytes)), true, nil
}

func (a *action) runUpgradeModule(
	c *cli.Context,
	mapImportedModules map[string]Module,
	successUpgradedModules []Module,
	modulePath string,
) ([]Module, error) {
	modulePath = strings.TrimSpace(modulePath)

	// Ignore empty
	if modulePath == "" {
		return successUpgradedModules, nil
	}

	// Ignore comment
	if strings.HasPrefix(modulePath, depsFileComment) {
		return successUpgradedModules, nil
	}

	a.log("Module path: %s\n", modulePath)

	// Ignore not imported module
	if _, ok := mapImportedModules[modulePath]; !ok {
		a.log("%s is not imported module\n", modulePath)
		return successUpgradedModules, nil
	}

	// Get module latest version
	goListArgs := []string{"list", "-m", "-u", "-json", modulePath}
	goOutput, err := exec.CommandContext(c.Context, "go", goListArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("failed to run go %+v: %w", strings.Join(goListArgs, " "), err)
	}
	a.log("Go output: %s\n", string(goOutput))

	module := Module{}
	if err := json.Unmarshal(goOutput, &module); err != nil {
		return successUpgradedModules, fmt.Errorf("failed to json unmarshal: %w", err)
	}
	a.log("Module: %+v\n", module)

	if module.Update == nil {
		color.PrintAppOK(name, fmt.Sprintf("You already have latest [%s] version [%s]", module.Path, module.Version))
		return successUpgradedModules, nil
	}

	// Upgrade module
	if a.flags.dryRun {
		// Only print which module will be upgraded
		// Don't do anything
		color.PrintAppOK(name, fmt.Sprintf("Will upgrade [%s] version [%s] to [%s]", module.Path, module.Version, module.Update.Version))
		return successUpgradedModules, nil
	}

	goGetArgs := []string{"get", "-d", modulePath + "@" + module.Update.Version}
	goOutput, err = exec.CommandContext(c.Context, "go", goGetArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("failed to run go %+v: %w", strings.Join(goGetArgs, " "), err)
	}
	a.log("Go output: %s\n", string(goOutput))

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
	a.log("Go output: %s\n", string(goOutput))

	if existVendor {
		// go mod vendor
		goModArgs = []string{"mod", "vendor"}
		goOutput, err = exec.CommandContext(c.Context, "go", goModArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
		}
		a.log("Go output: %s\n", string(goOutput))
	}

	return nil
}

func (a *action) runGitCommit(c *cli.Context, successUpgradedModules []Module, existVendor, useDepFile bool) error {
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
	if useDepFile {
		gitAddArgs = append(gitAddArgs, a.flags.depsFile)
	}

	gitOutput, err := exec.CommandContext(c.Context, "git", gitAddArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git %+v: %w", strings.Join(gitAddArgs, " "), err)
	}
	a.log("Git output: %s\n", string(gitOutput))

	// git commit
	gitCommitMessage := "build: upgrade modules\n"
	for _, module := range successUpgradedModules {
		gitCommitMessage += fmt.Sprintf("\n%s: %s -> %s", module.Path, module.Version, module.Update.Version)
	}
	gitCommitArgs := []string{"commit", "-m", gitCommitMessage}
	gitOutput, err = exec.CommandContext(c.Context, "git", gitCommitArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git %+v: %w", strings.Join(gitCommitArgs, " "), err)
	}
	a.log("Git output: %s\n", string(gitOutput))

	return nil
}
