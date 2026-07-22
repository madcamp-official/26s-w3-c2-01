// Package windowsdk detects installed Windows SDK / .NET Framework SDK
// versions under the Windows Kits install root.
//
// Note on platform isolation: docs/libra_collaboration_rules.md §8
// prescribes splitting Windows-only code into //go:build-tagged files
// (detector_windows.go / detector_unsupported.go). This package (and
// dotnet, msbuild's vswhere.go) instead uses a runtime
// adapter.RequireWindows(...) guard at the top of Detect, and builds fine
// on every platform. That's a deliberate difference, not a rule violation:
// none of these three call an actual Windows-only API (no syscalls, no
// registry) -- FilesystemDetector.Detect below just reads a directory path
// that happens to only exist on Windows -- so there's no non-portable
// surface that needs compile-time isolation in the first place.
package windowsdk

import (
	"context"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// Detector finds Windows SDK installations on this machine and reports them
// as domain.Resource values (Type == domain.ResourceTypeWindowsSDK).
type Detector interface {
	Detect(ctx context.Context) ([]domain.Resource, error)
}

// defaultKitsRoot is where Windows SDKs and the .NET Framework SDK install.
const defaultKitsRoot = `C:\Program Files (x86)\Windows Kits`

// FilesystemDetector finds SDKs under the Windows Kits install root:
//   - Windows 10/11 SDK: version-numbered subdirectories of 10\Include
//     (e.g. 10\Include\10.0.22621.0)
//   - Windows 8.1 SDK: the 8.1 directory itself, which -- unlike 10/11 --
//     has no version-numbered subdirectories of its own
//   - .NET Framework SDK: version-numbered subdirectories of NETFXSDK
//     (e.g. NETFXSDK\4.6.1)
//
// A missing root or subdirectory means that SDK isn't installed -- that is
// a valid result, not an error.
type FilesystemDetector struct {
	// KitsRoot overrides defaultKitsRoot. Used by tests; production callers
	// should leave it empty.
	KitsRoot string
}

func (d FilesystemDetector) root() string {
	if d.KitsRoot != "" {
		return d.KitsRoot
	}
	return defaultKitsRoot
}

func (d FilesystemDetector) Detect(ctx context.Context) ([]domain.Resource, error) {
	if err := adapter.RequireWindows("Windows SDK detection"); err != nil {
		return nil, err
	}
	var resources []domain.Resource

	win10, err := d.listVersionedResources(filepath.Join(d.root(), "10", "Include"), domain.ResourceTypeWindowsSDK, "Windows SDK")
	if err != nil {
		return nil, err
	}
	resources = append(resources, win10...)

	win81, err := d.detect81()
	if err != nil {
		return nil, err
	}
	resources = append(resources, win81...)

	netfx, err := d.listVersionedResources(filepath.Join(d.root(), "NETFXSDK"), domain.ResourceTypeNetFXSDK, ".NET Framework SDK")
	if err != nil {
		return nil, err
	}
	resources = append(resources, netfx...)

	return resources, nil
}

// listVersionedResources lists version-numbered subdirectories directly
// under dir (e.g. Include\10.0.22621.0, or NETFXSDK\4.6.1) and reports one
// Resource per version.
func (d FilesystemDetector) listVersionedResources(dir string, resourceType domain.ResourceType, label string) ([]domain.Resource, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var resources []domain.Resource
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		version := entry.Name()
		resource, err := newResource(resourceType, version, label+" "+version, filepath.Join(dir, version))
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// detect81 reports the Windows 8.1 SDK as a single resource: unlike 10/11,
// its install root has no version-numbered subdirectories -- the "8.1"
// directory itself is the one and only version.
func (d FilesystemDetector) detect81() ([]domain.Resource, error) {
	dir := filepath.Join(d.root(), "8.1")
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	resource, err := newResource(domain.ResourceTypeWindowsSDK, "8.1", "Windows SDK 8.1", dir)
	if err != nil {
		return nil, err
	}
	return []domain.Resource{resource}, nil
}

// newResource builds a detected domain.Resource with its display path
// computed through the shared pathutil contract. ID and NormalizedPath are
// left for app.ResourceService to derive -- it recomputes both from
// DisplayPath unconditionally, so computing them here would only be
// discarded.
func newResource(resourceType domain.ResourceType, version, name, path string) (domain.Resource, error) {
	displayPath, err := pathutil.Absolute(path)
	if err != nil {
		return domain.Resource{}, err
	}
	return domain.Resource{
		Name:        name,
		Type:        resourceType,
		Version:     version,
		DisplayPath: displayPath,
		// Declared: detected by listing a well-known, conventional install
		// path (Windows Kits root), same tier as android-sdk's
		// cachepath.Resource. Not set previously, which left Classification
		// (and thus Overall()) at 0 for every windows-sdk/netfx-sdk resource.
		Confidence: domain.DefaultConfidence[domain.EvidenceDeclared],
	}, nil
}
