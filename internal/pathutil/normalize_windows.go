//go:build windows

package pathutil

import (
	"path/filepath"
	"strings"
)

// normalize.go의 Normalize가 호출하는 normalizePlatform의 Windows용
// 구현. normalize_other.go와 //go:build 태그로 짝을 이루는 파일이다.
// Windows 파일시스템은 대소문자를 구분하지 않으므로, 경로 비교/DB 식별용
// 정규화 시 filepath.Clean 이후 전체를 소문자로 변환해 동일 경로를
// 대소문자 차이와 무관하게 같은 값으로 취급한다.
func normalizePlatform(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
