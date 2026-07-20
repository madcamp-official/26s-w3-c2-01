//go:build windows

package scanner

import (
	"io/fs"
	"syscall"
)

// scanner.go가 루트 검증(prepareRoots)과 엔트리 분류(entryFromInfo)에서
// 심볼릭 링크와 함께 순회 대상에서 제외할지 판단할 때 쓰는
// isReparsePoint의 Windows용 구현이다. os.FileInfo.Sys()를
// syscall.Win32FileAttributeData로 캐스팅해 FILE_ATTRIBUTE_REPARSE_POINT
// 플래그를 검사함으로써 정션/심볼릭 링크 등 실제 reparse point 여부를
// 판정한다. reparse_other.go와 //go:build 태그로 짝을 이루는 파일이며,
// 스캐너는 Options.FollowReparsePoints 설정과 무관하게 reparse point를
// 따라가지 않도록 이 판정 결과를 사용한다.
func isReparsePoint(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
