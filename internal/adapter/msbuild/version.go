package msbuild

import (
	"strconv"
	"strings"
)

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
