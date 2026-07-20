package scanner

import (
	"context"
	"math"
	"time"
)

// resource_size.go는 scanner.go가 제공하는 범용 디렉터리 순회(Scanner,
// Options, Result)와는 다른 목적을 가진 상위 레벨 헬퍼를 담는다: 여러
// 루트를 한 번에 훑는 일반 스캔이 아니라, 리소스 탐지기(detector)가 이미
// 찾아낸 "단일 리소스 경로 하나"를 대상으로 논리 크기와 최종 수정 시각을
// 측정하는 MeasureResource를 제공한다. 내부적으로는 scanner.go의 Scanner
// 인터페이스를 재사용해 해당 경로 하나를 MaxDepth 무제한으로 스캔하고,
// 그 Result를 리소스 단위 통계(ResourceSize)로 변환한다.
// ResourceSize is the filesystem metadata collected for a known resource
// path. Issues are recoverable entry-level failures encountered while walking.
type ResourceSize struct {
	LogicalSize    int64
	SizeKnown      bool
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
		SizeKnown:      len(result.Issues) == 0 && result.RootsScanned == 1,
		FilesInspected: result.FilesInspected,
		Issues:         result.Issues,
	}
	if !latest.IsZero() {
		measured.LastModifiedAt = &latest
	}
	return measured, nil
}
