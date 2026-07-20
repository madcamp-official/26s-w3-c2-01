//go:build !windows

package pathutil

import "path/filepath"

// normalize.go의 Normalize가 호출하는 normalizePlatform의 비Windows용
// 구현. normalize_windows.go와 //go:build 태그로 짝을 이루는 파일이다.
// macOS/Linux는 대소문자를 구분하는 파일시스템을 쓰는 것이 일반적이므로
// 별도의 대소문자 정규화 없이 filepath.Clean만 적용한다.
func normalizePlatform(path string) string {
	return filepath.Clean(path)
}
