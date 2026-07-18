package dotnet

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
