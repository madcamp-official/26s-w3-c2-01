package projectmarker

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// Detector recognizes build-tool manifests for ecosystems whose existing
// adapters previously exposed only global caches/SDKs.
type Detector struct{}

func (Detector) Detect(_ context.Context, entry scanner.Entry) (domain.BuildProject, bool, error) {
	if entry.Kind != scanner.EntryFile {
		return domain.BuildProject{}, false, nil
	}
	name := strings.ToLower(filepath.Base(entry.Path))
	var projectType domain.ProjectType
	switch name {
	case "pom.xml":
		projectType = domain.ProjectTypeMaven
	case "cargo.toml":
		projectType = domain.ProjectTypeCargo
	case "go.mod":
		projectType = domain.ProjectTypeGo
	case "build.gradle", "build.gradle.kts":
		projectType = domain.ProjectTypeGradle
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			return domain.BuildProject{}, true, err
		}
		text := string(data)
		if strings.Contains(text, "com.android.application") || strings.Contains(text, "com.android.library") {
			projectType = domain.ProjectTypeAndroid
		}
	default:
		return domain.BuildProject{}, false, nil
	}
	root := filepath.Dir(entry.Path)
	return domain.BuildProject{Name: filepath.Base(root), Type: projectType, RootPath: root, ManifestPath: entry.Path, LastModifiedAt: entry.ModifiedAt}, true, nil
}
