package msbuild

import (
	"path/filepath"
	"strings"
)

// abs := "D:\testdata\msbuild\GameClient\GameClient.vcxproj"  // filepath.Abs로 절대경로화
// root  = filepath.Dir(abs)                              // "D:\testdata\msbuild\GameClient"  (파일 뺀 나머지 폴더 경로)
// base  = filepath.Base(abs)                              // "GameClient.vcxproj"              (경로에서 파일명만)
// name  = strings.TrimSuffix(base, filepath.Ext(base))    // "GameClient"                       (확장자 뗀 이름)
// drive = filepath.VolumeName(abs)                         // "D:"                               (드라이브 문자)

func ProjectRoot(markerPath string) (root, name, drive string) {
	abs, err := filepath.Abs(markerPath)
	if err != nil {
		abs = markerPath
	}
	root = filepath.Dir(abs)
	base := filepath.Base(abs)
	name = strings.TrimSuffix(base, filepath.Ext(base))
	drive = filepath.VolumeName(abs)
	return root, name, drive
}
