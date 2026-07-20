// Package adapter contains contracts shared by platform-specific adapters.
package adapter

import (
	"errors"
	"fmt"
	"runtime"
)

// 이 파일은 adapter 패키지 자체의 유일한(테스트 제외) 소스 파일로, 여러 Windows 전용
// 어댑터 패키지(windowsdk, dotnet, msbuild의 vswhere.go)가 공통으로 가져다 쓰는 아주
// 작은 런타임 플랫폼 체크 헬퍼 하나(RequireWindows)만 담고 있다. 이 세 어댑터는
// //go:build 태그로 파일을 컴파일타임에 나누는 대신, 탐지 함수 진입부에서 이 함수를 호출해
// Windows가 아닌 플랫폼에서는 "설치된 게 없다"는 빈 성공 결과가 아니라 명시적인 에러를
// 반환하도록 만든다 -- 그래야 macOS/Linux에서 돌렸을 때 실제로는 지원되지 않는 기능이
// 조용히 빈 목록을 돌려주며 정상 동작한 것처럼 보이는 상황을 막을 수 있다.

var ErrUnsupportedPlatform = errors.New("unsupported platform")

// RequireWindows returns a descriptive error instead of making a Windows-only
// adapter look like a successful empty detection on another platform.
func RequireWindows(feature string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return fmt.Errorf("%w: %s requires Windows (current: %s)", ErrUnsupportedPlatform, feature, runtime.GOOS)
}
