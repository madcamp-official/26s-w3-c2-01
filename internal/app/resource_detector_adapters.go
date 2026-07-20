// [파일 역할] internal/adapter/{windowsdk,dotnet,msbuild} 각각의 시스템 리소스
// 탐지 타입(windowsdk.Detector, dotnet.SDKLister, msbuild.ToolLocator)을
// analysis_contract.go의 ResourceDetector 인터페이스로 감싸는 어댑터 모음이다.
// project_detector_adapters.go의 "리소스 버전" 대응 파일이라고 볼 수 있는데,
// 차이는 파일 순회 중 만나는 scanner.Entry가 아니라 (현재는 빈 구조체인)
// Environment를 받아 시스템 전역(설치된 SDK/툴 등)을 한 번에 스캔한다는 점이다.
// cmd/scan.go가 이 구조체들을 생성해 AnalysisOrchestrator.WithDetectors에
// 주입하고, analysis_orchestrator.go의 AnalysisOrchestrator.Run이
// DISCOVER_SYSTEM_RESOURCES 단계에서 호출한다.
package app

import (
	"context"
	"errors"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
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
