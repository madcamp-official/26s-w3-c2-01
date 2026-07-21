package output

import (
	"fmt"
	"io"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type PurgeView struct {
	TransactionID string                          `json:"transaction_id"`
	DryRun        bool                            `json:"dry_run"`
	Status        domain.CleanupTransactionStatus `json:"status"`
	Candidates    []string                        `json:"candidates"`
}

// Envelope maps PurgeView onto the shared JSON envelope (issue #59),
// reusing the same status mapping as CleanupTransactionView since both
// carry domain.CleanupTransactionStatus. Per-candidate detail isn't wired
// as Issues here -- Candidates is just a path list with no per-item
// success/failure of its own to report (unlike a transaction's items).
func (v PurgeView) Envelope() EnvelopeOptions {
	return EnvelopeOptions{Outcome: transactionOutcome(v.Status)}
}

func (v PurgeView) RenderText(w io.Writer) error {
	mode := "DRY RUN"
	if !v.DryRun {
		mode = "EXECUTED"
	}
	fmt.Fprintf(w, "Purge %s: %s\nStatus: %s\n", mode, v.TransactionID, v.Status)
	for _, path := range v.Candidates {
		fmt.Fprintf(w, "- %s\n", path)
	}
	return nil
}
