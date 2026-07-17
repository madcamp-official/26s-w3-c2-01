package windowsdk

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// Detector finds Windows SDK installations on this machine and reports them
// as domain.Resource values (Type == domain.ResourceTypeWindowsSDK).
type Detector interface {
	Detect(ctx context.Context) ([]domain.Resource, error)
}
