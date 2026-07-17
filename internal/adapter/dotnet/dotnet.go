package dotnet

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// SDKLister finds installed .NET SDKs, typically by parsing the output of
// `dotnet --list-sdks`, and reports them as domain.Resource values
// (Type == domain.ResourceTypeDotNetSDK).
type SDKLister interface {
	ListSDKs(ctx context.Context) ([]domain.Resource, error)
}
