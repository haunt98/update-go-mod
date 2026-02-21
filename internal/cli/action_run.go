package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/urfave/cli/v3"

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
	ErrGoModExistToolchain  = errors.New("go mod exist toolchain")
)

func (a *action) Run(ctx context.Context, c *cli.Command) error {
	a.getFlags(c)

	mapImportedModules, err := a.runGetImportedModules(ctx)
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
	successUpgradedModules := make([]*Module, 0, defaultCountModule)
	modulePaths := strings.SplitSeq(depsStr, "\n")
	for modulePath := range modulePaths {
		successUpgradedModules, err = a.runUpgradeModule(
			ctx,
			mapImportedModules,
			successUpgradedModules,
			modulePath,
		)
		if err != nil {
			color.PrintAppError(name, err.Error())
		}
	}

	if err := a.runGoMod(ctx, existVendor); err != nil {
		return err
	}

	if err := a.runGitCommit(ctx, successUpgradedModules, existVendor, useDepFile); err != nil {
		return err
	}

	return nil
}

// Get all imported modules
func (a *action) runGetImportedModules(ctx context.Context) (map[string]*Module, error) {
	goListAllArgs := []string{"list", "-m", "-json", "-mod=readonly", "all"}
	goOutput, err := exec.CommandContext(ctx, "go", goListAllArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("exec: failed to run go %s: %w", strings.Join(goListAllArgs, " "), err)
	}

	// goAllOutput is like {...}\n{...}\n{...}
	// Missing [] and , for json
	goOutputStr := strings.ReplaceAll(strings.TrimSpace(string(goOutput)), "\n", "")
	goOutputStr = strings.ReplaceAll(goOutputStr, "}{", "},{")
	goOutputStr = "[" + goOutputStr + "]"

	importedModules := make([]*Module, 0, defaultCountModule)
	if err := sonic.UnmarshalString(goOutputStr, &importedModules); err != nil {
		return nil, fmt.Errorf("sonic: failed to unmarshal: %w", err)
	}
	a.log("Go output: %s\n", string(goOutput))

	mapImportedModules := make(map[string]*Module)
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
			return "", false, fmt.Errorf("url: failed to parse %s: %w", a.flags.depsURL, err)
		}

		// nolint:noctx
		httpRsp, err := http.Get(depsURL.String())
		if err != nil {
			return "", false, fmt.Errorf("http: failed to get %s: %w", depsURL.String(), err)
		}
		defer httpRsp.Body.Close()

		if httpRsp.StatusCode != http.StatusOK {
			return "", false, fmt.Errorf("http: status code not ok %d: %w", httpRsp.StatusCode, ErrFailedStatusCode)
		}

		depsBytes, err := io.ReadAll(httpRsp.Body)
		if err != nil {
			return "", false, fmt.Errorf("io: failed to read all: %w", err)
		}

		return strings.TrimSpace(string(depsBytes)), false, nil
	}

	// If empty url, try to read from file
	depsBytes, err := os.ReadFile(a.flags.depsFile)
	if err != nil {
		if os.IsNotExist(err) {
			color.PrintAppWarning(name, fmt.Sprintf("[%s] not found", a.flags.depsFile))
			return "", false, nil
		}

		return "", false, fmt.Errorf("os: failed to read file %s: %w", a.flags.depsFile, err)
	}

	return strings.TrimSpace(string(depsBytes)), true, nil
}

func (a *action) runUpgradeModule(
	ctx context.Context,
	mapImportedModules map[string]*Module,
	successUpgradedModules []*Module,
	modulePath string,
) ([]*Module, error) {
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
	goListArgs := []string{"list", "-m", "-u", "-json", "-mod=readonly", modulePath}
	goOutput, err := exec.CommandContext(ctx, "go", goListArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goListArgs, " "), err)
	}
	a.log("Go output: %s\n", string(goOutput))

	module := &Module{}
	if err := sonic.Unmarshal(goOutput, module); err != nil {
		return successUpgradedModules, fmt.Errorf("sonic: failed to unmarshal: %w", err)
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

	goGetArgs := []string{"get", modulePath + "@" + module.Update.Version}
	goOutput, err = exec.CommandContext(ctx, "go", goGetArgs...).CombinedOutput()
	if err != nil {
		return successUpgradedModules, fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goGetArgs, " "), err)
	}
	a.log("Go output: %s\n", string(goOutput))

	successUpgradedModules = append(successUpgradedModules, module)

	color.PrintAppOK(name, fmt.Sprintf("Upgraded [%s] version [%s] to [%s] success", module.Path, module.Version, module.Update.Version))

	return successUpgradedModules, nil
}

func (a *action) runGoMod(ctx context.Context, existVendor bool) error {
	if a.flags.dryRun {
		return nil
	}

	// go mod edit -toolchain=none
	goModArgs := []string{"mod", "edit", "-toolchain=none"}
	if _, err := exec.CommandContext(ctx, "go", goModArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
	}

	// go mod tidy
	goModArgs = []string{"mod", "tidy"}
	goOutput, err := exec.CommandContext(ctx, "go", goModArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
	}
	a.log("Go output: %s\n", string(goOutput))

	if existVendor {
		// go mod vendor
		goModArgs = []string{"mod", "vendor"}
		goOutput, err = exec.CommandContext(ctx, "go", goModArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
		}
		a.log("Go output: %s\n", string(goOutput))
	}

	goMod, err := a.runReadGoMod(ctx)
	if err != nil {
		return err
	}

	if goMod.Toolchain != "" {
		return ErrGoModExistToolchain
	}

	return nil
}

func (a *action) runReadGoMod(ctx context.Context) (*GoMod, error) {
	goModArgs := []string{"mod", "edit", "-json"}
	goOutput, err := exec.CommandContext(ctx, "go", goModArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("exec: failed to run go %+v: %w", strings.Join(goModArgs, " "), err)
	}

	goMod := &GoMod{}
	if err := sonic.Unmarshal(goOutput, goMod); err != nil {
		return nil, fmt.Errorf("sonic: failed to unmarshal: %w", err)
	}
	a.log("Go output: %s\n", string(goOutput))

	return goMod, nil
}

func (a *action) runGitCommit(ctx context.Context, successUpgradedModules []*Module, existVendor, useDepFile bool) error {
	if a.flags.dryRun {
		return nil
	}

	// If not exist git, stop
	if _, err := os.Stat(gitDirectory); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("os: failed to stat %s: %w", gitDirectory, err)
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

	gitOutput, err := exec.CommandContext(ctx, "git", gitAddArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec: failed to run git %+v: %w", strings.Join(gitAddArgs, " "), err)
	}
	a.log("Git output: %s\n", string(gitOutput))

	// git commit
	var gitCommitMessage strings.Builder
	gitCommitMessage.WriteString("build: upgrade modules\n")
	for _, module := range successUpgradedModules {
		fmt.Fprintf(&gitCommitMessage, "\n%s: %s -> %s", module.Path, module.Version, module.Update.Version)
	}
	gitCommitArgs := []string{"commit", "-m", gitCommitMessage.String()}
	gitOutput, err = exec.CommandContext(ctx, "git", gitCommitArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec: failed to run git %+v: %w", strings.Join(gitCommitArgs, " "), err)
	}
	a.log("Git output: %s\n", string(gitOutput))

	return nil
}
