package scanner

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// ignore.go는 config의 exclude 목록(절대 경로/상대 경로 패턴)을 실제 파일
// 경로와 비교해 "이 경로는 스캔에서 제외해야 하는가"를 판정하는
// excludeMatcher를 정의한다. 아래 영어 주석대로 경로 매칭(절대/상대 패턴
// 구분, 대소문자·구분자 정규화)은 디렉터리를 실제로 순회하는 로직과는
// 다른 관심사이기 때문에 scanner.go의 walk 코드와 분리했다. scanner.go는
// newExcludeMatcher로 이 파일의 excludeMatcher를 만들어 각 엔트리를 방문할
// 때마다 Matches를 호출해 사용한다.
// excludeMatcher implements the config exclude-list check scanner.go's walk
// consults per entry -- split into its own file since path-matching (absolute
// vs. relative exclude patterns, normalization) is a distinct concern from
// the walk/visitor logic in scanner.go itself.
type excludeMatcher struct {
	roots    []string
	absolute []string
	relative []string
}

func newExcludeMatcher(roots, excludes []string) (excludeMatcher, error) {
	matcher := excludeMatcher{roots: make([]string, 0, len(roots))}
	for _, root := range roots {
		normalized, err := pathutil.Normalize(root)
		if err != nil {
			return excludeMatcher{}, err
		}
		matcher.roots = append(matcher.roots, normalized)
	}

	for _, exclude := range excludes {
		if strings.TrimSpace(exclude) == "" {
			continue
		}
		if filepath.IsAbs(exclude) {
			normalized, err := pathutil.Normalize(exclude)
			if err != nil {
				return excludeMatcher{}, err
			}
			matcher.absolute = append(matcher.absolute, normalized)
			continue
		}
		matcher.relative = append(matcher.relative, canonical(filepath.Clean(exclude)))
	}

	return matcher, nil
}

func (m excludeMatcher) Matches(path string) bool {
	normalized, err := pathutil.Normalize(path)
	if err != nil {
		return false
	}
	for _, excluded := range m.absolute {
		if sameOrChild(normalized, excluded) {
			return true
		}
	}
	for _, root := range m.roots {
		relative, err := filepath.Rel(root, normalized)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		relative = canonical(relative)
		for _, excluded := range m.relative {
			if sameOrChild(relative, excluded) {
				return true
			}
		}
	}
	return false
}

func canonical(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func sameOrChild(path, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+string(filepath.Separator))
}
