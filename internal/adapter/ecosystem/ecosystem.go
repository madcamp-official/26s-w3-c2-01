// Package ecosystem detects read-only SDK and global cache locations for
// Android/Gradle, Cargo, Maven, npm, and pnpm.
package ecosystem

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type ResourceLister interface {
	ListResources(context.Context) ([]domain.Resource, error)
}

type Paths struct {
	Home string
	Env  func(string) string
	Stat func(string) (os.FileInfo, error)
}

func (p Paths) env(key string) string {
	if p.Env != nil {
		return p.Env(key)
	}
	return os.Getenv(key)
}
func (p Paths) home() (string, error) {
	if p.Home != "" {
		return p.Home, nil
	}
	return os.UserHomeDir()
}
func (p Paths) directory(path string) bool {
	stat := p.Stat
	if stat == nil {
		stat = os.Stat
	}
	info, err := stat(path)
	return err == nil && info.IsDir()
}
func resource(name, version, path string, resourceType domain.ResourceType, confidence int) domain.Resource {
	return domain.Resource{Name: name, Version: version, Type: resourceType, DisplayPath: path, Confidence: confidence}
}

type AndroidGradleLister struct {
	Paths Paths
	GOOS  string
}

func (l AndroidGradleLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Paths.home()
	if err != nil {
		return nil, err
	}
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	sdk := l.Paths.env("ANDROID_HOME")
	if sdk == "" {
		sdk = l.Paths.env("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		switch goos {
		case "windows":
			sdk = filepath.Join(l.Paths.env("LOCALAPPDATA"), "Android", "Sdk")
		case "darwin":
			sdk = filepath.Join(home, "Library", "Android", "sdk")
		default:
			sdk = filepath.Join(home, "Android", "Sdk")
		}
	}
	gradleHome := l.Paths.env("GRADLE_USER_HOME")
	if gradleHome == "" {
		gradleHome = filepath.Join(home, ".gradle")
	}
	var out []domain.Resource
	if sdk != "" && l.Paths.directory(sdk) {
		out = append(out, resource("Android SDK", "android-sdk", sdk, domain.ResourceTypeAndroidSDK, domain.DefaultConfidence[domain.EvidenceDeclared]))
	}
	cache := filepath.Join(gradleHome, "caches")
	if l.Paths.directory(cache) {
		out = append(out, resource("Gradle global cache", "gradle", cache, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared]))
	}
	return out, nil
}

type CargoLister struct{ Paths Paths }

func (l CargoLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Paths.home()
	if err != nil {
		return nil, err
	}
	cargoHome := l.Paths.env("CARGO_HOME")
	if cargoHome == "" {
		cargoHome = filepath.Join(home, ".cargo")
	}
	var out []domain.Resource
	for _, item := range []struct{ name, version, sub string }{{"Cargo registry cache", "cargo-registry", "registry"}, {"Cargo git cache", "cargo-git", "git"}} {
		path := filepath.Join(cargoHome, item.sub)
		if l.Paths.directory(path) {
			out = append(out, resource(item.name, item.version, path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared]))
		}
	}
	return out, nil
}

type MavenLister struct {
	Paths    Paths
	ReadFile func(string) ([]byte, error)
}

func (l MavenLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Paths.home()
	if err != nil {
		return nil, err
	}
	repository := filepath.Join(home, ".m2", "repository")
	readFile := l.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	data, err := readFile(filepath.Join(home, ".m2", "settings.xml"))
	if err == nil {
		var settings struct {
			LocalRepository string `xml:"localRepository"`
		}
		if err := xml.Unmarshal(data, &settings); err != nil {
			return nil, fmt.Errorf("decode Maven settings.xml: %w", err)
		}
		if value := strings.TrimSpace(settings.LocalRepository); value != "" {
			repository = strings.ReplaceAll(value, "${user.home}", home)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read Maven settings.xml: %w", err)
	}
	if !l.Paths.directory(repository) {
		return nil, nil
	}
	return []domain.Resource{resource("Maven local repository", "maven", repository, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}

type NodeCacheLister struct {
	Tool     string
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) ([]byte, error)
}

func (l NodeCacheLister) ListResources(ctx context.Context) ([]domain.Resource, error) {
	look := l.LookPath
	if look == nil {
		look = exec.LookPath
	}
	run := l.Run
	if run == nil {
		run = func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, path, args...).Output()
		}
	}
	tool := l.Tool
	if tool == "" {
		tool = "npm"
	}
	args, name := []string{"config", "get", "cache"}, "npm global cache"
	if tool == "pnpm" {
		args, name = []string{"store", "path"}, "pnpm global store"
	} else if tool != "npm" {
		return nil, fmt.Errorf("unsupported node cache tool %q", tool)
	}
	path, err := look(tool)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value, err := run(ctx, path, args...)
	if err != nil {
		return nil, fmt.Errorf("%s cache location: %w", tool, err)
	}
	cachePath := strings.TrimSpace(string(value))
	if cachePath == "" {
		return nil, nil
	}
	if info, err := os.Stat(cachePath); err == nil && info.IsDir() {
		return []domain.Resource{resource(name, tool, cachePath, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceResolved])}, nil
	}
	return nil, nil
}
