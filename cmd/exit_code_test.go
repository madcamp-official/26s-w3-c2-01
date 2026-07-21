package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestExitCodeContract(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{fmt.Errorf("wrapped: %w", sqlite.ErrCleanupPlanNotFound), ExitNotFound},
		{fmt.Errorf("wrapped: %w", safety.ErrCleanupBlocked), ExitSafetyBlocked},
		{fmt.Errorf("cancelled: %w", ErrUserCancelled), ExitCancelled},
		{errors.New("disk failure"), ExitInternal},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.err); got != tt.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

func TestPartialTransactionExitCode(t *testing.T) {
	resultExitCode = ExitSuccess
	recordTransactionExit(domain.TransactionPartiallyRestored)
	if got := ExitCode(nil); got != ExitPartialCleanup {
		t.Fatalf("ExitCode(nil) = %d, want %d", got, ExitPartialCleanup)
	}
	resultExitCode = ExitSuccess
	recordTransactionExit(domain.TransactionFailed)
	if got := ExitCode(nil); got != ExitSafetyBlocked {
		t.Fatalf("failed transaction ExitCode(nil) = %d, want %d", got, ExitSafetyBlocked)
	}
}
