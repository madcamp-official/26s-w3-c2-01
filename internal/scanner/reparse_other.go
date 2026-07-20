//go:build !windows

package scanner

import "io/fs"

// scanner.go가 루트 검증(prepareRoots)과 엔트리 분류(entryFromInfo)에서
// 심볼릭 링크와 함께 순회 대상에서 제외할지 판단할 때 쓰는
// isReparsePoint의 비Windows용 구현이다. reparse point는 Windows 고유
// 개념(정션/심볼릭 링크 등)이라 다른 OS에서는 판정할 방법이 없으므로
// 항상 false를 반환한다. reparse_windows.go와 //go:build 태그로 짝을
// 이루는 파일.
func isReparsePoint(fs.FileInfo) bool {
	return false
}
