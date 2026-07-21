// Package conda detects conda environments: globally registered named
// environments (via `conda env list --json`) and local prefix environments
// created directly under a project root (`conda create -p ./envs`).
//
// Scope for this iteration (see docs/libra_integration_contracts.md §19.4/
// §19.5, docs/libra_python_conda_scope_decisions.md 결정 4/5/7/9):
//
//   - A globally registered named environment is a REQUIRES-only shared
//     resource -- never a cleanup candidate, regardless of RiskPolicy
//     output. It is connected to a project only when the project's
//     environment.yml "name" field matches the environment's actual name
//     (see DeclaredEnvironmentName; the app-layer dependency analyzer does
//     the matching).
//   - A local prefix environment (conda-meta/history found directly under a
//     project root) is the one exception: its location alone is ownership
//     evidence, so it is treated as an OWNS resource, the same as a Node
//     node_modules or Python venv.
//   - conda not being installed (no `conda`/`conda.bat` on PATH) is a valid
//     "no environments" result, not an error -- same contract as
//     internal/adapter/dotnet.CLISDKLister, but platform-independent (conda
//     runs on Windows/macOS/Linux alike, so this package carries no
//     adapter.RequireWindows guard).
//   - pip/conda global package caches are out of scope; see 결정 9.
package conda

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"go.yaml.in/yaml/v3"
)

// EnvironmentFile and environmentFileAlt are the recognized conda
// environment manifest names, checked in that order.
const (
	EnvironmentFile     = "environment.yml"
	environmentFileAlt  = "environment.yaml"
	defaultCondaCommand = "conda"
	condaMetaHistory    = "conda-meta/history"
)

// EnvLister finds globally registered conda environments and reports them as
// domain.Resource values (Type == domain.ResourceTypeCondaEnv).
type EnvLister interface {
	ListEnvs(ctx context.Context) ([]domain.Resource, error)
}

// CLIEnvLister finds conda environments by running `conda env list --json`.
// A conda executable missing from PATH means no conda installation -- that
// is a valid result (nil, nil), not an error (결정 7).
type CLIEnvLister struct {
	// CondaPath overrides the PATH lookup. Used by tests; production callers
	// should leave it empty.
	CondaPath string
	// LookPath overrides exec.LookPath. Used by tests to deterministically
	// simulate "conda not installed" without depending on the machine
	// actually running the test suite.
	LookPath func(file string) (string, error)
	// Run executes the conda command and returns its stdout. Overridable for
	// tests; defaults to actually running the command.
	Run func(ctx context.Context, path string, args ...string) ([]byte, error)
}

func (l CLIEnvLister) path() (string, bool) {
	if l.CondaPath != "" {
		return l.CondaPath, true
	}
	lookup := l.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	found, err := lookup(defaultCondaCommand)
	if err != nil {
		return "", false
	}
	return found, true
}

func (l CLIEnvLister) run(ctx context.Context, path string, args ...string) ([]byte, error) {
	if l.Run != nil {
		return l.Run(ctx, path, args...)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (l CLIEnvLister) ListEnvs(ctx context.Context) ([]domain.Resource, error) {
	condaPath, found := l.path()
	if !found {
		return nil, nil
	}
	output, err := l.run(ctx, condaPath, "env", "list", "--json")
	if err != nil {
		return nil, err
	}
	return parseEnvList(output)
}

type envListOutput struct {
	Envs []string `json:"envs"`
}

// parseEnvList parses `conda env list --json`'s {"envs": ["/path", ...]}
// shape into domain.Resource values.
func parseEnvList(output []byte) ([]domain.Resource, error) {
	var parsed envListOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, err
	}
	resources := make([]domain.Resource, 0, len(parsed.Envs))
	for _, envPath := range parsed.Envs {
		resource, err := newCondaEnvResource(envPath)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// newCondaEnvResource builds a detected domain.Resource with its display
// path computed through the shared pathutil contract. ID and NormalizedPath
// are left for app.ResourceService to derive, same as dotnet's SDK lister.
func newCondaEnvResource(path string) (domain.Resource, error) {
	displayPath, err := pathutil.Absolute(path)
	if err != nil {
		return domain.Resource{}, err
	}
	return domain.Resource{
		Name:        envNameFromPath(displayPath),
		Type:        domain.ResourceTypeCondaEnv,
		DisplayPath: displayPath,
	}, nil
}

// envNameFromPath derives the environment's name the way `conda env list`
// implies it: environments registered under an "envs" directory are named
// after their own basename; the root/base install (which conda always lists
// first, outside any "envs" directory) is named "base".
func envNameFromPath(path string) string {
	if filepath.Base(filepath.Dir(path)) == "envs" {
		return filepath.Base(path)
	}
	return "base"
}

// genericEnvNames and genericEnvNamePattern are names common enough across
// unrelated projects that a match against them is weak evidence of a real
// project-specific relationship (docs/libra_python_conda_scope_decisions.md
// 결정 5: "일반적인 이름일 때는 REVIEW로 강등").
var genericEnvNames = map[string]struct{}{
	"base": {}, "env": {}, "root": {}, "test": {}, "venv": {},
}

var genericEnvNamePattern = regexp.MustCompile(`^(py|python)\d+$`)

// IsGenericEnvName reports whether name is too generic to trust as a
// project-specific conda environment reference on its own.
func IsGenericEnvName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if _, ok := genericEnvNames[lower]; ok {
		return true
	}
	return genericEnvNamePattern.MatchString(lower)
}

// DeclaredEnvironmentName reads the "name" field from root's
// environment.yml/environment.yaml, if present. This is the declared-
// dependency marker the app-layer dependency analyzer matches against
// EnvLister's results to build a PROJECT REQUIRES RESOURCE edge.
//
// A missing manifest is a valid "nothing declared" result (ok == false), not
// an error, matching every other adapter's manifest-optional contract.
func DeclaredEnvironmentName(root string) (name, sourcePath string, ok bool, err error) {
	for _, candidate := range []string{EnvironmentFile, environmentFileAlt} {
		path := filepath.Join(root, candidate)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return "", "", false, readErr
		}
		var doc struct {
			Name string `yaml:"name"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", "", false, err
		}
		if strings.TrimSpace(doc.Name) == "" {
			return "", "", false, nil
		}
		return doc.Name, path, true, nil
	}
	return "", "", false, nil
}

// DetectLocalPrefixEnvs finds conda environments created directly under a
// project root (`conda create -p ./envs`, immediate children only -- conda
// environments are not nested). Unlike a globally registered named
// environment, these are OWNS resources: the location itself, not a name
// match, is the ownership evidence (결정 5 예외).
//
// hasEnvironmentFile marks whether root also declares an environment.yml --
// the only regeneration evidence a local prefix env can have, since there is
// no separate lockfile mechanism for conda the way poetry.lock/uv.lock work
// for Python packaging.
func DetectLocalPrefixEnvs(root string, hasEnvironmentFile bool) ([]domain.Resource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var resources []domain.Resource
	for _, entry := range entries {
		candidate := filepath.Join(root, entry.Name())
		if !entry.IsDir() {
			info, statErr := os.Stat(candidate)
			if statErr != nil || !info.IsDir() {
				continue
			}
		}
		if _, err := os.Stat(filepath.Join(candidate, filepath.FromSlash(condaMetaHistory))); err != nil {
			continue
		}
		resource := domain.Resource{
			Name:        entry.Name(),
			Type:        domain.ResourceTypeCondaEnv,
			DisplayPath: candidate,
			Regenerable: hasEnvironmentFile,
			Confidence:  domain.DefaultConfidence[domain.EvidenceObserved],
		}
		if hasEnvironmentFile {
			resource.RegenerationCommand = "conda env create -f " + EnvironmentFile
		}
		resources = append(resources, resource)
	}
	return resources, nil
}
