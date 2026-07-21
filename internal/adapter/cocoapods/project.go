package cocoapods

import (
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// DetectArtifacts reports a project-owned `Pods/` directory (installed pods
// for the project rooted at root) as a domain.Resource, distinct from the
// global CocoaPods download cache CacheLister reports. A `Podfile` sibling
// is required -- a bare `Pods/` directory with no Podfile isn't something
// CocoaPods would recreate here, so it isn't reported as this project's
// artifact. `Podfile.lock` is the regeneration evidence, the same role
// package-lock.json plays for node_modules.
func DetectArtifacts(root string) ([]domain.Resource, error) {
	if _, err := os.Stat(filepath.Join(root, "Podfile")); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	podsPath := filepath.Join(root, "Pods")
	info, err := os.Stat(podsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	regenerable := false
	confidence := domain.DefaultConfidence[domain.EvidenceInferred]
	if _, err := os.Stat(filepath.Join(root, "Podfile.lock")); err == nil {
		regenerable = true
		confidence = domain.DefaultConfidence[domain.EvidenceDeclared]
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	resource := domain.Resource{
		Name:        "Pods",
		Type:        domain.ResourceTypePods,
		DisplayPath: podsPath,
		Regenerable: regenerable,
		Confidence:  confidence,
	}
	if regenerable {
		resource.RegenerationCommand = "pod install"
	}
	return []domain.Resource{resource}, nil
}
