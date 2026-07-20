// Package git detects Git repositories (a bare .git entry) as a fallback
// BuildProject classification -- see Detector's doc comment below for why
// it must only be used when no stronger project marker exists at the same
// root.
package git

import (
	"context"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// 이 파일은 git 패키지의 유일한 소스 파일이며, Detector 인터페이스와 그 실제 구현체
// FilesystemDetector를 정의한다. 하는 일은 "이 디렉터리에 .git이 있는가"만 확인해서
// domain.BuildProject로 만드는 것뿐인 아주 작은 탐지기지만, node/msbuild 패키지 소속이
// 아니라 별도 패키지로 분리되어 있다 -- 특정 빌드 생태계에 속하지 않고 .vcxproj/.csproj/
// package.json 등 더 강한 마커가 하나도 없을 때만 최후 수단(fallback)으로 쓰이는,
// 모든 프로젝트 탐지기 공통의 최하위 우선순위 로직이기 때문이다. 이 우선순위 규칙(더 강한
// 마커가 있으면 git.go는 호출조차 되면 안 됨)을 지키는 책임은 호출자에게 있다 -- 그렇지
// 않으면 같은 디렉터리가 msbuild-cpp/git 두 개의 BuildProject로 중복 등록되어
// LogicalSize가 이중으로 집계된다.

// Detector determines whether a directory entry is the root of a Git
// repository and, if so, builds the resulting domain.BuildProject.
//
// This is a fallback classification: callers should only invoke it for a
// directory that has no other recognized project marker (.vcxproj, .csproj,
// package.json). Otherwise the same directory would be registered as two
// BuildProject rows (e.g. both msbuild-cpp and git) with the same disk
// footprint counted under each, double-counting its LogicalSize.
type Detector interface {
	// CanDetect reports whether entry's directory contains a .git entry.
	// .git is normally a directory, but in a linked worktree it is a file
	// pointing at the real repository elsewhere -- either form counts.
	CanDetect(entry scanner.Entry) bool
	// Detect builds the domain.BuildProject(s) for the Git repository rooted
	// at entry. Callers should only call this after CanDetect reports true.
	// It returns a slice, rather than a single BuildProject, so that a Git
	// root containing more than one independent build project is not
	// precluded by the return type.
	Detect(ctx context.Context, entry scanner.Entry) ([]domain.BuildProject, error)
}

// FilesystemDetector is the real Detector implementation: it checks for a
// .git entry directly on disk, so it needs no mocking for tests.
type FilesystemDetector struct{}

func (FilesystemDetector) CanDetect(entry scanner.Entry) bool {
	_, err := os.Stat(filepath.Join(entry.Path, ".git"))
	return err == nil
}

func (FilesystemDetector) Detect(ctx context.Context, entry scanner.Entry) ([]domain.BuildProject, error) {
	abs, err := pathutil.Absolute(entry.Path)
	if err != nil {
		return nil, err
	}

	return []domain.BuildProject{{
		Name:           filepath.Base(abs),
		Type:           domain.ProjectTypeGit,
		RootPath:       abs,
		ManifestPath:   filepath.Join(abs, ".git"),
		Drive:          filepath.VolumeName(abs),
		LastModifiedAt: entry.ModifiedAt,
	}}, nil
}
