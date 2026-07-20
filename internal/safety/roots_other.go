//go:build !windows

package safety

// path_classifier.go의 NewSystemPathClassifier가 호출하는
// systemProtectedRoots()의 비Windows용 구현이며, roots_windows.go와
// //go:build 태그로 짝을 이룬다. 아래 영어 주석대로 현재는 MVP 범위상
// macOS/Linux용 보호 루트가 정의되어 있지 않아 항상 nil을 반환한다(버그가
// 아니라 의도된 범위 축소).
// systemProtectedRoots returns no protected paths on non-Windows: MVP scope
// (docs/libra_integration_contracts.md §20.3) only defines protection for
// Windows env vars (%WINDIR% etc, see roots_windows.go) since Windows is
// libra's primary target. This is a scope decision, not a bug -- macOS/
// Linux system paths simply aren't classified as SystemManaged yet.
func systemProtectedRoots() []string {
	return nil
}
