package app

import (
	"context"
	"errors"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/conda"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/docker"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/dotnet"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/windowsdk"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// WindowsSDKResourceDetector adapts windowsdk.Detector to ResourceDetector.
// The adapter carries its own configuration (windowsdk.FilesystemDetector.
// KitsRoot), so Environment is unused -- unlike project-scoped resources
// (see project_detector_adapters.go's package doc), system-wide resource
// detectors need no per-call input to know what to scan.
type WindowsSDKResourceDetector struct{ Detector windowsdk.Detector }

func (d WindowsSDKResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Detector == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Detector.Detect(ctx)
	if err != nil {
		return resourceDetectionFailure("windowsdk", err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

// DotNetSDKResourceDetector adapts dotnet.SDKLister to ResourceDetector.
type DotNetSDKResourceDetector struct{ Lister dotnet.SDKLister }

func (d DotNetSDKResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Lister == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Lister.ListSDKs(ctx)
	if err != nil {
		return resourceDetectionFailure("dotnet", err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

type EcosystemResourceDetector struct {
	Name   string
	Lister interface {
		ListResources(context.Context) ([]domain.Resource, error)
	}
}

func (d EcosystemResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Lister == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Lister.ListResources(ctx)
	if err != nil {
		return resourceDetectionFailure(d.Name, err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

// DockerResourceDetector reads aggregate daemon usage. The underlying adapter
// is deliberately read-only and never invokes prune.
type DockerResourceDetector struct{ Lister docker.UsageLister }

func (d DockerResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Lister == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Lister.ListUsage(ctx)
	if err != nil {
		return resourceDetectionFailure("docker", err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

// VisualStudioResourceDetector adapts msbuild.ToolLocator (vswhere-based) to
// ResourceDetector.
type VisualStudioResourceDetector struct{ Locator msbuild.ToolLocator }

func (d VisualStudioResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Locator == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Locator.Locate(ctx)
	if err != nil {
		return resourceDetectionFailure("msbuild", err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

// CondaResourceDetector adapts conda.EnvLister to ResourceDetector. It only
// reports globally registered environments -- local prefix environments
// under a project root come through PythonProjectDetector instead (§19.4/
// §19.5 결정 4·5).
type CondaResourceDetector struct{ Lister conda.EnvLister }

func (d CondaResourceDetector) Detect(ctx context.Context, _ Environment) DetectionResult[domain.Resource] {
	if d.Lister == nil {
		return DetectionResult[domain.Resource]{}
	}
	resources, err := d.Lister.ListEnvs(ctx)
	if err != nil {
		return resourceDetectionFailure("conda", err)
	}
	return DetectionResult[domain.Resource]{Items: resources}
}

func resourceDetectionFailure(adapterName string, err error) DetectionResult[domain.Resource] {
	code := IssueAdapterFailed
	if errors.Is(err, adapter.ErrUnsupportedPlatform) {
		code = IssueUnsupportedPlatform
	}
	return DetectionResult[domain.Resource]{
		Issues: []Issue{{Code: code, Phase: PhaseDiscoverSystemResources, Adapter: adapterName,
			Operation: "detect resources", Severity: IssueWarning, Message: err.Error(), Cause: err}},
	}
}
