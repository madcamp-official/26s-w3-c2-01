package swiftpm

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

const manifestName = "Package.swift"

// ToolsVersionPropertyName is the ProjectProperty.Name
// app.XcodeDependencyAnalyzer looks for -- the swift-tools-version comment
// every Package.swift must start with, e.g. "// swift-tools-version:5.9".
// It is the SwiftPM analogue of MSBuild's WindowsTargetPlatformVersion: a
// real declared version requirement, just not one this codebase resolves
// against an installed Swift toolchain version (xcodebuild -version reports
// the Xcode version, not the Swift tools version it ships).
const ToolsVersionPropertyName = "swift-tools-version"

// Detector recognizes a Package.swift manifest as the root of a SwiftPM
// BuildProject.
type Detector struct{}

func (Detector) CanDetect(entry scanner.Entry) bool {
	return entry.Kind == scanner.EntryFile && filepath.Base(entry.Path) == manifestName
}

func (Detector) Detect(_ context.Context, entry scanner.Entry) (domain.BuildProject, error) {
	root := filepath.Dir(entry.Path)
	return domain.BuildProject{
		Name:           filepath.Base(root),
		Type:           domain.ProjectTypeSwiftPM,
		RootPath:       root,
		ManifestPath:   entry.Path,
		LastModifiedAt: entry.ModifiedAt,
	}, nil
}

// ToolsVersion reads the declared "// swift-tools-version:X.Y" comment,
// which every valid Package.swift must have on its first line. Returns ""
// if the file can't be read or doesn't start with the expected comment --
// treated as "not declared", not an error, since a malformed manifest here
// shouldn't abort project detection (matching the rest of this codebase's
// recoverable-failure convention).
func ToolsVersion(manifestPath string) string {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return ""
	}
	line := strings.TrimSpace(scanner.Text())
	const prefix = "// swift-tools-version:"
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(line, prefix))
}

// DetectArtifacts reports the project-owned `.build/` directory SwiftPM
// creates alongside Package.swift -- the SwiftPM analogue of node_modules,
// a build.System.build_output that name-matches ResourceTypeBuildOutput
// exactly like MSBuild's bin/obj do. `Package.resolved` (SwiftPM's lockfile)
// is the regeneration evidence.
func DetectArtifacts(root string) ([]domain.Resource, error) {
	buildPath := filepath.Join(root, ".build")
	info, err := os.Stat(buildPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	regenerable := false
	confidence := domain.DefaultConfidence[domain.EvidenceInferred]
	if _, err := os.Stat(filepath.Join(root, "Package.resolved")); err == nil {
		regenerable = true
		confidence = domain.DefaultConfidence[domain.EvidenceDeclared]
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	resource := domain.Resource{
		Name:        ".build",
		Type:        domain.ResourceTypeBuildOutput,
		DisplayPath: buildPath,
		Regenerable: regenerable,
		Confidence:  confidence,
	}
	if regenerable {
		resource.RegenerationCommand = "swift build"
	}
	return []domain.Resource{resource}, nil
}
