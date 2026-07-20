//go:build windows

package safety

import "os"

// path_classifier.go의 NewSystemPathClassifier가 호출하는
// systemProtectedRoots()의 Windows용 구현이며, roots_other.go와
// //go:build 태그로 짝을 이룬다. WINDIR, ProgramFiles,
// ProgramFiles(x86), ProgramData 환경 변수를 읽어 실제 시스템 디렉터리
// 경로를 보호 루트 목록으로 구성한다. Libra의 주 타깃이 Windows이기
// 때문에 이 목록이 채워지는 쪽은 여기뿐이다.
func systemProtectedRoots() []string {
	names := []string{"WINDIR", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"}
	roots := make([]string, 0, len(names))
	for _, name := range names {
		if root := os.Getenv(name); root != "" {
			roots = append(roots, root)
		}
	}
	return roots
}
