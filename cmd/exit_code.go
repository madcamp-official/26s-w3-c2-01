package cmd

import (
	"errors"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

const (
	ExitSuccess        = 0
	ExitNotFound       = 2
	ExitInternal       = 3
	ExitSafetyBlocked  = 4
	ExitPartialCleanup = 5
	ExitCancelled      = 130
)

var ErrUserCancelled = errors.New("operation cancelled")

var resultExitCode int

func ExitCode(err error) int {
	if err == nil {
		return resultExitCode
	}
	switch {
	case errors.Is(err, ErrUserCancelled):
		return ExitCancelled
	case errors.Is(err, ErrTargetNotFound), errors.Is(err, sqlite.ErrResourceNotFound),
		errors.Is(err, sqlite.ErrProjectNotFound), errors.Is(err, sqlite.ErrScanNotFound),
		errors.Is(err, sqlite.ErrCleanupPlanNotFound), errors.Is(err, sqlite.ErrCleanupTransactionNotFound):
		return ExitNotFound
	case errors.Is(err, safety.ErrCleanupBlocked):
		return ExitSafetyBlocked
	default:
		return ExitInternal
	}
}

func recordTransactionExit(status domain.CleanupTransactionStatus) {
	switch status {
	case domain.TransactionPartiallyQuarantined, domain.TransactionPartiallyRestored, domain.TransactionPartiallyPurged:
		resultExitCode = ExitPartialCleanup
	case domain.TransactionFailed:
		// Cleanup transactions reach FAILED when no candidate passes the
		// revalidation/restore guards. The operation was safely refused.
		resultExitCode = ExitSafetyBlocked
	}
}
