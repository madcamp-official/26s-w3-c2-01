// Package safety classifies filesystem paths before cleanup policy is applied.
package safety

import (
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// path_classifier.go는 임의의 경로가 정리(cleanup) 정책을 적용해서는 안 되는
// "시스템이 관리하는" 보호 영역 아래에 있는지를 판정하는 PathClassifier를
// 정의한다. 실제로 어떤 경로들이 보호 루트인지(예: Windows의 %WINDIR%,
// %ProgramFiles% 등)는 OS마다 다르므로 이 파일이 직접 나열하지 않고
// systemProtectedRoots() 함수로 위임하며, 그 구현은 //go:build 태그로 나뉜
// roots_windows.go와 roots_other.go에 각각 있다. 이 파일은 순수하게 "보호
// 루트 목록이 주어졌을 때 어떻게 분류하는가"라는 플랫폼 독립적인 로직만
// 담당한다.
type PathClassification struct {
	SystemManaged bool
	ProtectedRoot string
}

type PathClassifier struct {
	protectedRoots []string
}

// NewPathClassifier builds a classifier from explicit protected roots.
func NewPathClassifier(protectedRoots []string) (*PathClassifier, error) {
	normalized := make([]string, 0, len(protectedRoots))
	seen := make(map[string]struct{}, len(protectedRoots))
	for _, root := range protectedRoots {
		identity, err := pathutil.Normalize(root)
		if err != nil {
			return nil, fmt.Errorf("normalize protected root %q: %w", root, err)
		}
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		normalized = append(normalized, identity)
	}
	return &PathClassifier{protectedRoots: normalized}, nil
}

// NewSystemPathClassifier uses the protected operating-system roots known on
// the current host.
func NewSystemPathClassifier() (*PathClassifier, error) {
	return NewPathClassifier(systemProtectedRoots())
}

func (c *PathClassifier) Classify(path string) (PathClassification, error) {
	for _, root := range c.protectedRoots {
		protected, err := pathutil.IsSameOrChild(path, root)
		if err != nil {
			return PathClassification{}, fmt.Errorf("classify path %q: %w", path, err)
		}
		if protected {
			return PathClassification{SystemManaged: true, ProtectedRoot: root}, nil
		}
	}
	return PathClassification{}, nil
}
