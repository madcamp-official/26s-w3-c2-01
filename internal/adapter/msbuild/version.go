package msbuild

import (
	"strconv"
	"strings"
)

// 이 파일은 점(.)으로 구분된 버전 문자열("10.0.22621.0" 같은)을 다루는 순수 파싱/비교
// 유틸(parseVersion, hasVersionPrefix, compareVersions)만 모아둔 파일이다. domain이나
// scanner 등 다른 패키지에 전혀 의존하지 않는 순수 함수들이라 resolve.go의 매칭 정책
// 로직과 분리해 두었고, resolve.go의 MatchWindowsSDK/MatchDotNetSDK가 "선언된 버전과
// 설치된 버전이 맞는지, 어느 쪽이 더 높은 버전인지"를 판단할 때 이 파일의 함수들을
// 그대로 가져다 쓴다. 문자열 그대로 비교하면 "9000"이 "19041"보다 크다고 잘못 판단하는
// 문제가 있어서, 반드시 정수 세그먼트로 쪼개 숫자로 비교한다.

// parseVersion splits a dotted version string (e.g. "10.0.22621.0") into
// integer segments so it can be compared numerically instead of
// lexicographically (a string compare would wrongly rank "9000" above
// "19041").
func parseVersion(version string) ([]int, bool) {
	parts := strings.Split(version, ".")
	segments := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		segments[i] = n
	}
	return segments, true
}

// hasVersionPrefix reports whether version starts with all of prefix's
// segments, e.g. [10 0 22621 0] has prefix [10 0].
func hasVersionPrefix(version, prefix []int) bool {
	if len(prefix) > len(version) {
		return false
	}
	for i, p := range prefix {
		if version[i] != p {
			return false
		}
	}
	return true
}

// compareVersions returns -1, 0, or 1 as a < b, a == b, or a > b, comparing
// segment by segment.
func compareVersions(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
