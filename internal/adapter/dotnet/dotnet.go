// Package dotnet detects installed .NET SDKs by shelling out to the
// official `dotnet` CLI (SDKLister) rather than guessing install
// directories, per docs/libra_cli_commands_and_schedule.md's F-04
// ("공식 .NET CLI가 설치된 SDK와 Runtime 목록을 제공하므로 직접 설치 폴더만
// 추측하지 않고 명령 결과를 우선 사용한다"). Originally gated to Windows via
// adapter.RequireWindows even though `dotnet` itself is cross-platform --
// that was a product-scope decision (docs/libra_cli_commands_and_schedule.md
// 우선 지원 OS), not a technical limitation, and is now lifted: the `dotnet`
// CLI's own output format is identical on every OS, only the well-known
// install path differs (see resolvePath).
package dotnet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

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
	// DotnetPath overrides the resolved dotnet executable path. Used by
	// tests (and matches the historical Windows default install location);
	// production callers should leave it empty.
	DotnetPath string
	// LookPath resolves "dotnet" on PATH when DotnetPath is empty and GOOS
	// isn't windows (which has its own well-known install path instead, see
	// resolvePath). Overridable for tests; defaults to exec.LookPath.
	LookPath func(string) (string, error)
	// Run executes the dotnet command and returns its stdout. Overridable
	// for tests; defaults to actually running the command.
	Run func(ctx context.Context, path string, args ...string) ([]byte, error)
}

// resolvePath finds the dotnet executable. An explicit DotnetPath override
// or Windows's well-known install location is checked with os.Stat, exactly
// as before. Without an override on non-Windows there is no single
// well-known path to hardcode the way Windows has one (the official
// installer, Homebrew, and Linux distro packages all place it differently),
// so dotnet is resolved via PATH instead -- the same approach every other
// CLI-based ecosystem adapter here uses (npm/pnpm/homebrew).
func (l CLISDKLister) resolvePath() (string, error) {
	path := l.DotnetPath
	if path == "" && runtime.GOOS == "windows" {
		path = defaultDotnetPath
	}
	if path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "", nil
		} else if err != nil {
			return "", err
		}
		return path, nil
	}
	look := l.LookPath
	if look == nil {
		look = exec.LookPath
	}
	resolved, err := look("dotnet")
	if errors.Is(err, exec.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return resolved, nil
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
	dotnetPath, err := l.resolvePath()
	if err != nil {
		return nil, err
	}
	if dotnetPath == "" {
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
