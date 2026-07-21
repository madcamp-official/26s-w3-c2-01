package app

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type fakeWindowsSDKDetector struct {
	resources []domain.Resource
	err       error
}

func (f fakeWindowsSDKDetector) Detect(context.Context) ([]domain.Resource, error) {
	return f.resources, f.err
}

type fakeDotNetSDKLister struct {
	resources []domain.Resource
	err       error
}

func (f fakeDotNetSDKLister) ListSDKs(context.Context) ([]domain.Resource, error) {
	return f.resources, f.err
}

type fakeToolLocator struct {
	resources []domain.Resource
	err       error
}

func (f fakeToolLocator) Locate(context.Context) ([]domain.Resource, error) {
	return f.resources, f.err
}

type fakeCondaEnvLister struct {
	resources []domain.Resource
	err       error
}

type fakeDockerUsageLister struct {
	resources []domain.Resource
	err       error
}

func (f fakeDockerUsageLister) ListUsage(context.Context) ([]domain.Resource, error) {
	return f.resources, f.err
}

func (f fakeCondaEnvLister) ListEnvs(context.Context) ([]domain.Resource, error) {
	return f.resources, f.err
}

func TestResourceDetectorAdaptersPassThroughResources(t *testing.T) {
	resources := []domain.Resource{{Type: domain.ResourceTypeWindowsSDK, Version: "10.0.22621.0"}}

	got := WindowsSDKResourceDetector{Detector: fakeWindowsSDKDetector{resources: resources}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("WindowsSDKResourceDetector.Detect() = %#v", got)
	}

	got = DotNetSDKResourceDetector{Lister: fakeDotNetSDKLister{resources: resources}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("DotNetSDKResourceDetector.Detect() = %#v", got)
	}

	got = VisualStudioResourceDetector{Locator: fakeToolLocator{resources: resources}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("VisualStudioResourceDetector.Detect() = %#v", got)
	}

	condaResources := []domain.Resource{{Type: domain.ResourceTypeCondaEnv, Name: "base"}}
	got = CondaResourceDetector{Lister: fakeCondaEnvLister{resources: condaResources}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("CondaResourceDetector.Detect() = %#v", got)
	}

	dockerResources := []domain.Resource{{Type: domain.ResourceTypeDockerCache, Name: "Docker Images"}}
	got = DockerResourceDetector{Lister: fakeDockerUsageLister{resources: dockerResources}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("DockerResourceDetector.Detect() = %#v", got)
	}
}

func TestResourceDetectorAdaptersReportUnsupportedPlatformAsRecoverableIssue(t *testing.T) {
	err := adapter.RequireWindows("test feature")
	if err == nil {
		t.Skip("running on windows: RequireWindows does not fail here")
	}

	got := WindowsSDKResourceDetector{Detector: fakeWindowsSDKDetector{err: err}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 0 || len(got.Issues) != 1 || got.Issues[0].Code != IssueUnsupportedPlatform {
		t.Fatalf("Detect() = %#v, want a single UNSUPPORTED_PLATFORM issue", got)
	}
}

func TestResourceDetectorAdaptersReportOtherFailuresAsAdapterFailed(t *testing.T) {
	got := DotNetSDKResourceDetector{Lister: fakeDotNetSDKLister{err: errors.New("boom")}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 0 || len(got.Issues) != 1 || got.Issues[0].Code != IssueAdapterFailed {
		t.Fatalf("Detect() = %#v, want a single ADAPTER_FAILED issue", got)
	}
}

func TestDockerResourceDetectorReportsDaemonFailureAsRecoverableIssue(t *testing.T) {
	got := DockerResourceDetector{Lister: fakeDockerUsageLister{err: errors.New("daemon unavailable")}}.Detect(context.Background(), Environment{})
	if len(got.Items) != 0 || len(got.Issues) != 1 || got.Issues[0].Adapter != "docker" {
		t.Fatalf("Detect() = %#v, want Docker ADAPTER_FAILED issue", got)
	}
}
