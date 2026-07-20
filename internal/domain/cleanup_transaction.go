package domain

import "time"

type CleanupTransactionStatus string
type CleanupTransactionItemStatus string

const (
	TransactionPlanned              CleanupTransactionStatus = "PLANNED"
	TransactionRunning              CleanupTransactionStatus = "RUNNING"
	TransactionQuarantined          CleanupTransactionStatus = "QUARANTINED"
	TransactionPartiallyQuarantined CleanupTransactionStatus = "PARTIALLY_QUARANTINED"
	TransactionRestored             CleanupTransactionStatus = "RESTORED"
	TransactionPartiallyRestored    CleanupTransactionStatus = "PARTIALLY_RESTORED"
	TransactionPurged               CleanupTransactionStatus = "PURGED"
	TransactionFailed               CleanupTransactionStatus = "FAILED"

	TransactionItemPending  CleanupTransactionItemStatus = "PENDING"
	TransactionItemMoved    CleanupTransactionItemStatus = "MOVED"
	TransactionItemSkipped  CleanupTransactionItemStatus = "SKIPPED"
	TransactionItemFailed   CleanupTransactionItemStatus = "FAILED"
	TransactionItemRestored CleanupTransactionItemStatus = "RESTORED"
)

type CleanupTransaction struct {
	ID         string
	PlanID     string
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     CleanupTransactionStatus
	Items      []CleanupTransactionItem
}

type CleanupTransactionItem struct {
	ID             string
	PlanItemID     string
	ResourceID     string
	OriginalPath   string
	QuarantinePath string
	ManifestPath   string
	ExpectedSize   int64
	Status         CleanupTransactionItemStatus
	Reason         string
}
