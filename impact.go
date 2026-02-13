package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TestResult struct {
	Passed   bool
	Duration time.Duration
}

func RunImpact(cfg *Config) error {
	// Setup workspace temp dir
	workspace, err := os.MkdirTemp("", "otel-impact-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)

	fmt.Println("Workspace:", workspace)

	// Clone latest released version of target
	releasedPath := filepath.Join(workspace, "released")
	if err := cloneRepo(cfg.Target.RepoURL, cfg.Target.ReleasedRef, releasedPath); err != nil {
		return err
	}

	// Path of modified local version of target
	modifiedPath := cfg.Target.ModifiedLocalPath

	for _, dep := range cfg.Dependents {
		fmt.Println("\n================================================")
		fmt.Println("Dependent:", dep.RepoURL)
		fmt.Println("Module   :", dep.ModulePath)
		fmt.Println("================================================")

		if err := runForDependent(
			workspace,
			dep,
			releasedPath,
			modifiedPath,
			cfg.Target.ModulePrefix,
		); err != nil {
			return err
		}
	}

	return nil
}

func runForDependent(
	workspace string,
	dep DependentConfig,
	released string,
	modified string,
	modulePrefix string,
) error {

	depRepoPath := filepath.Join(workspace, "dependent-repo")
	if err := cloneRepo(dep.RepoURL, dep.Ref, depRepoPath); err != nil {
		return err
	}

	modulePath := filepath.Join(depRepoPath, dep.ModulePath)

	fmt.Println("\n--- Baseline ---")
	baseline := runTests(modulePath)

	fmt.Println("\n--- Released ---")
	if err := applyModuleReplacements(modulePath, released, modulePrefix); err != nil {
		return err
	}
	releasedRes := runTests(modulePath)

	fmt.Println("\n--- Modified ---")
	if err := applyModuleReplacements(modulePath, modified, modulePrefix); err != nil {
		return err
	}
	modifiedRes := runTests(modulePath)

	fmt.Println("\nSummary:")
	fmt.Println("Baseline :", passFail(baseline))
	fmt.Println("Released :", passFail(releasedRes))
	fmt.Println("Modified :", passFail(modifiedRes))
	fmt.Println("Outcome  :", classify(releasedRes.Passed, modifiedRes.Passed))

	return nil
}

// Shallow clone of main branch of dependent module repo without history for faster execution
func cloneRepo(url, ref, dest string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func applyModuleReplacements(dir, repoPath, modulePrefix string) error {
	fmt.Println("Applying replacements for prefix:", modulePrefix)

	// Drop prev replace directives
	drop := exec.Command("go", "mod", "edit", "-dropreplace=all")
	drop.Dir = dir
	if err := drop.Run(); err != nil {
		return fmt.Errorf("failed to drop replaces: %w", err)
	}

	// Reset module graph
	tidyBefore := exec.Command("go", "mod", "tidy")
	tidyBefore.Dir = dir
	if err := tidyBefore.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	modules, err := listModules(dir)
	if err != nil {
		return err
	}

	found := false

	for _, m := range modules {
		if strings.HasPrefix(m, modulePrefix) {

			rel := strings.TrimPrefix(m, "go.opentelemetry.io/otel")
			local := filepath.Join(repoPath, rel)

			if _, err := os.Stat(filepath.Join(local, "go.mod")); err == nil {
				fmt.Println("  replace", m, "=>", local)

				cmd := exec.Command(
					"go", "mod", "edit",
					fmt.Sprintf("-replace=%s=%s", m, local),
				)
				cmd.Dir = dir

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("replace failed for %s: %w", m, err)
				}

				found = true
			}
		}
	}

	if !found {
		fmt.Println("  WARNING: no modules matched prefix:", modulePrefix)
	}

	tidyAfter := exec.Command("go", "mod", "tidy")
	tidyAfter.Dir = dir
	if err := tidyAfter.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}

func listModules(dir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var modules []string
	dec := json.NewDecoder(stdout)

	for {
		var m struct {
			Path string
		}
		if err := dec.Decode(&m); err != nil {
			break
		}
		modules = append(modules, m.Path)
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return modules, nil
}

func runTests(dir string) TestResult {
	start := time.Now()

	// count to avoid caching of test results
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	err := cmd.Run()

	res := TestResult{
		Passed:   err == nil,
		Duration: time.Since(start),
	}

	if res.Passed {
		fmt.Println("✔ PASS")
	} else {
		fmt.Println("✘ FAIL")
	}

	fmt.Println("Duration:", res.Duration.Round(time.Second))
	return res
}

func passFail(r TestResult) string {
	if r.Passed {
		return "PASS"
	}
	return "FAIL"
}

func classify(released, modified bool) string {
	switch {
	case released && modified:
		return "UNCHANGED"
	case released && !modified:
		return "REGRESSION"
	case !released && modified:
		return "IMPROVEMENT"
	default:
		return "UNCHANGED-FAIL"
	}
}
