package scanner

import (
	"context"
	"math"
	"time"
)

// ResourceSize is the filesystem metadata collected for a known resource
// path. Issues are recoverable entry-level failures encountered while walking.
type ResourceSize struct {
	LogicalSize    int64
	FilesInspected int64
	LastModifiedAt *time.Time
	Issues         []Issue
}

// MeasureResource walks one detector-provided resource path without following
// symlinks or Windows reparse points.
func MeasureResource(ctx context.Context, walker Scanner, path string) (ResourceSize, error) {
	var latest time.Time
	result, err := walker.Scan(ctx, Options{
		Roots:    []string{path},
		MaxDepth: math.MaxInt,
	}, func(_ context.Context, entry Entry) error {
		if entry.ModifiedAt.After(latest) {
			latest = entry.ModifiedAt
		}
		return nil
	})
	if err != nil {
		return ResourceSize{}, err
	}

	measured := ResourceSize{
		LogicalSize:    result.LogicalSize,
		FilesInspected: result.FilesInspected,
		Issues:         result.Issues,
	}
	if !latest.IsZero() {
		measured.LastModifiedAt = &latest
	}
	return measured, nil
}
