// Package safety classifies filesystem paths before cleanup policy is applied.
package safety

import (
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

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
