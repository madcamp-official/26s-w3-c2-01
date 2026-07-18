package msbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// defaultVSWherePath is where the Visual Studio Installer places vswhere.exe.
// It ships automatically with any Visual Studio 2017+ installation.
const defaultVSWherePath = `C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe`

// vswhereInstance is the subset of vswhere.exe's JSON output libra needs.
type vswhereInstance struct {
	InstallationPath    string `json:"installationPath"`
	InstallationVersion string `json:"installationVersion"`
	DisplayName         string `json:"displayName"`
}

// VSWhereToolLocator finds Visual Studio installations by running
// vswhere.exe and parsing its JSON output. A missing vswhere.exe means no
// Visual Studio is installed -- that is a valid result, not an error.
type VSWhereToolLocator struct {
	// VSWherePath overrides defaultVSWherePath. Used by tests; production
	// callers should leave it empty.
	VSWherePath string
	// Run executes vswhere.exe and returns its stdout. Overridable for
	// tests; defaults to actually running the command.
	Run func(ctx context.Context, path string, args ...string) ([]byte, error)
}

func (l VSWhereToolLocator) path() string {
	if l.VSWherePath != "" {
		return l.VSWherePath
	}
	return defaultVSWherePath
}

func (l VSWhereToolLocator) run(ctx context.Context, path string, args ...string) ([]byte, error) {
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

func (l VSWhereToolLocator) Locate(ctx context.Context) ([]domain.Resource, error) {
	vswhere := l.path()
	if _, err := os.Stat(vswhere); os.IsNotExist(err) {
		return nil, nil
	}

	output, err := l.run(ctx, vswhere, "-format", "json", "-utf8")
	if err != nil {
		return nil, err
	}

	var instances []vswhereInstance
	if err := json.Unmarshal(output, &instances); err != nil {
		return nil, err
	}

	resources := make([]domain.Resource, 0, len(instances))
	for _, inst := range instances {
		resource, err := newVSResource(inst.DisplayName, inst.InstallationVersion, inst.InstallationPath)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// newVSResource builds a domain.Resource with its ID and both path forms
// computed through the shared pathutil contract, rather than normalizing
// paths independently.
func newVSResource(name, version, path string) (domain.Resource, error) {
	displayPath, err := pathutil.Absolute(path)
	if err != nil {
		return domain.Resource{}, err
	}
	normalizedPath, err := pathutil.Normalize(path)
	if err != nil {
		return domain.Resource{}, err
	}
	return domain.Resource{
		ID:             domain.ResourceID(domain.ResourceTypeVisualStudio, version, normalizedPath),
		Name:           name,
		Type:           domain.ResourceTypeVisualStudio,
		Version:        version,
		DisplayPath:    displayPath,
		NormalizedPath: normalizedPath,
	}, nil
}
