// Package dotnet detects installed .NET SDKs by shelling out to the
// official `dotnet` CLI (SDKLister) rather than guessing install
// directories, per docs/libra_cli_commands_and_schedule.md's F-04
// ("공식 .NET CLI가 설치된 SDK와 Runtime 목록을 제공하므로 직접 설치 폴더만
// 추측하지 않고 명령 결과를 우선 사용한다"). Windows-only in practice, guarded
// by adapter.RequireWindows rather than a //go:build tag -- see
// windowsdk.FilesystemDetector's doc comment for why that's the pattern
// this whole codebase uses instead of platform-specific files here.
package dotnet

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// 이 파일은 dotnet 패키지의 유일한 소스 파일로, ".NET SDK 설치 목록 조회"라는 책임 하나만
// 진다. 레지스트리나 설치 폴더를 직접 뒤지는 대신 공식 `dotnet --list-sdks` CLI를 실행해 그
// 출력(예: "8.0.404 [C:\Program Files\dotnet\sdk]")을 파싱하는데, 이는 SDK 설치 방식이
// 바뀌어도 libra가 직접 따라가지 않고 .NET 툴체인이 보장하는 안정적인 인터페이스에 기대기
// 위함이다. SDKLister 인터페이스, 실제 구현체 CLISDKLister, 그리고 출력 파싱 로직
// (parseListSDKs/splitSDKLine)이 모두 이 한 파일 안에 들어 있다. Windows 전용 기능이지만
// windowsdk 패키지와 마찬가지로 //go:build 태그로 파일을 분리하지 않고 ListSDKs 안에서
// adapter.RequireWindows로 런타임에 걸러낸다 -- 레지스트리/syscall 같은 실제 플랫폼 종속
// API를 쓰지 않고 dotnet.exe 존재 여부만 확인하므로 컴파일타임 분리가 필요 없기 때문이다.

// SDKLister finds installed .NET SDKs, typically by parsing the output of
// `dotnet --list-sdks`, and reports them as domain.Resource values
// (Type == domain.ResourceTypeDotNetSDK).
type SDKLister interface {
	ListSDKs(ctx context.Context) ([]domain.Resource, error)
}

// defaultDotnetPath is where the .NET SDK installer places dotnet.exe.
const defaultDotnetPath = `C:\Program Files\dotnet\dotnet.exe`

// CLISDKLister finds .NET SDKs by running `dotnet --list-sdks` and parsing
// its output. A missing dotnet executable means no .NET SDK is installed --
// that is a valid result, not an error.
type CLISDKLister struct {
	// DotnetPath overrides defaultDotnetPath. Used by tests; production
	// callers should leave it empty.
	DotnetPath string
	// Run executes the dotnet command and returns its stdout. Overridable
	// for tests; defaults to actually running the command.
	Run func(ctx context.Context, path string, args ...string) ([]byte, error)
}

func (l CLISDKLister) path() string {
	if l.DotnetPath != "" {
		return l.DotnetPath
	}
	return defaultDotnetPath
}

func (l CLISDKLister) run(ctx context.Context, path string, args ...string) ([]byte, error) {
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

func (l CLISDKLister) ListSDKs(ctx context.Context) ([]domain.Resource, error) {
	if err := adapter.RequireWindows("Windows .NET SDK detection"); err != nil {
		return nil, err
	}
	dotnetPath := l.path()
	if _, err := os.Stat(dotnetPath); os.IsNotExist(err) {
		return nil, nil
	}

	output, err := l.run(ctx, dotnetPath, "--list-sdks")
	if err != nil {
		return nil, err
	}

	return parseListSDKs(output)
}

// parseListSDKs parses lines like:
//
//	8.0.404 [C:\Program Files\dotnet\sdk]
//
// into domain.Resource values.
func parseListSDKs(output []byte) ([]domain.Resource, error) {
	var resources []domain.Resource
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		version, dir, ok := splitSDKLine(line)
		if !ok {
			continue
		}
		resource, err := newDotNetSDKResource(version, filepath.Join(dir, version))
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// newDotNetSDKResource builds a detected domain.Resource with its display
// path computed through the shared pathutil contract. ID and NormalizedPath
// are left for app.ResourceService to derive -- it recomputes both from
// DisplayPath unconditionally, so computing them here would only be
// discarded.
func newDotNetSDKResource(version, path string) (domain.Resource, error) {
	displayPath, err := pathutil.Absolute(path)
	if err != nil {
		return domain.Resource{}, err
	}
	return domain.Resource{
		Name:        ".NET SDK " + version,
		Type:        domain.ResourceTypeDotNetSDK,
		Version:     version,
		DisplayPath: displayPath,
	}, nil
}

func splitSDKLine(line string) (version, dir string, ok bool) {
	openIdx := strings.Index(line, "[")
	closeIdx := strings.LastIndex(line, "]")
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return "", "", false
	}
	version = strings.TrimSpace(line[:openIdx])
	dir = strings.TrimSpace(line[openIdx+1 : closeIdx])
	if version == "" || dir == "" {
		return "", "", false
	}
	return version, dir, true
}
