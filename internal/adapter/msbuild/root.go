package msbuild

import (
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// 이 파일은 "marker 파일 경로(.sln/.vcxproj/.csproj) -> 프로젝트 루트/이름/드라이브"라는
// 아주 작지만 여러 파서가 똑같이 필요로 하는 로직 하나(ProjectRoot)만 따로 뗀 공용 유틸
// 파일이다. 지금은 xmlparser.go의 XMLBuildProjectParser가 쓰고 있지만, 앞으로 .sln을
// 파싱하는 WorkspaceParser 구현체가 추가되더라도 같은 함수를 그대로 재사용할 수 있도록
// 어느 한쪽 파서 파일에 묻어두지 않고 독립 파일로 분리해 두었다.

// ProjectRoot derives a build project's root directory, display name, and
// drive from the path of a marker file (a .sln, .vcxproj, or .csproj). The
// root is simply the marker file's containing directory -- nested marker
// files (e.g. a .vcxproj referenced by a .sln elsewhere) each get their own
// root independently of one another.
func ProjectRoot(markerPath string) (root, name, drive string, err error) {
	abs, err := pathutil.Absolute(markerPath)
	if err != nil {
		return "", "", "", err
	}
	root = filepath.Dir(abs)
	base := filepath.Base(abs)
	name = strings.TrimSuffix(base, filepath.Ext(base))
	drive = filepath.VolumeName(abs)
	return root, name, drive, nil
}
