package domain

import "time"

type CleanupPlanStatus string

const (
	CleanupPlanReady                  CleanupPlanStatus = "READY"
	CleanupPlanInsufficientCandidates CleanupPlanStatus = "INSUFFICIENT_CANDIDATES"
)

type CleanupPlan struct {
	ID            string
	CreatedAt     time.Time
	TargetBytes   int64
	SelectedBytes int64
	Status        CleanupPlanStatus
	Items         []CleanupPlanItem
}

type CleanupPlanItem struct {
	ID                   string
	ResourceID           string
	NormalizedPath       string
	ExpectedType         ResourceType
	ExpectedSize         int64
	ExpectedModifiedTime time.Time
	RiskAtPlanning       RiskLevel
	ConfidenceAtPlanning int
	OwnerProjectID       string
	ScanID               string
	RegenerationCommand  string
}
