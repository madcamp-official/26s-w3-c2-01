package msbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// 이 파일은 msbuild.go가 정의하는 ToolLocator 인터페이스의 실제 구현체
// (VSWhereToolLocator)로, Visual Studio 설치기가 함께 깔아주는 vswhere.exe를 실행해 그
// JSON 출력을 파싱함으로써 설치된 Visual Studio/MSBuild를 domain.Resource로 변환한다.
// vswhere.exe 자체가 없으면 "Visual Studio 미설치"로 보고 에러가 아닌 빈 결과를 반환한다.
// windowsdk, dotnet 패키지와 마찬가지로 //go:build 태그 대신 Locate 진입부의
// adapter.RequireWindows 런타임 체크만 쓰는데, 레지스트리나 syscall 같은 실제 Windows
// 전용 API를 호출하지 않고 exec.Command로 외부 실행 파일을 실행할 뿐이라 컴파일타임 분리가
// 필요 없기 때문이다.

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
	if err := adapter.RequireWindows("Visual Studio detection"); err != nil {
		return nil, err
	}
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

// newVSResource builds a detected domain.Resource with its display path
// computed through the shared pathutil contract. ID and NormalizedPath are
// left for app.ResourceService to derive -- it recomputes both from
// DisplayPath unconditionally, so computing them here would only be
// discarded.
func newVSResource(name, version, path string) (domain.Resource, error) {
	displayPath, err := pathutil.Absolute(path)
	if err != nil {
		return domain.Resource{}, err
	}
	return domain.Resource{
		Name:        name,
		Type:        domain.ResourceTypeVisualStudio,
		Version:     version,
		DisplayPath: displayPath,
	}, nil
}
