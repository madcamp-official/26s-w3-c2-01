package output

import (
	"fmt"
	"io"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type CleanupTransactionItemView struct {
	OriginalPath   string                              `json:"original_path"`
	QuarantinePath string                              `json:"quarantine_path"`
	Status         domain.CleanupTransactionItemStatus `json:"status"`
	Reason         string                              `json:"reason,omitempty"`
}
type CleanupTransactionView struct {
	ID     string                          `json:"id"`
	PlanID string                          `json:"plan_id"`
	Status domain.CleanupTransactionStatus `json:"status"`
	Items  []CleanupTransactionItemView    `json:"items"`
}
type CleanupTransactionsView struct {
	Transactions []CleanupTransactionView `json:"transactions"`
}

// transactionOutcome maps a domain.CleanupTransactionStatus onto the shared
// JSON envelope's Outcome (issue #59). Shared by CleanupTransactionView
// (clean --execute, restore) and PurgeView, which both carry the same
// status type. PLANNED/RUNNING should never actually reach a Printer call
// (RunE only prints after Execute/Purge returns a terminal transaction),
// but map to PARTIAL rather than SUCCESS if one ever does, since neither
// means the operation actually finished.
func transactionOutcome(status domain.CleanupTransactionStatus) Outcome {
	switch status {
	case domain.TransactionQuarantined, domain.TransactionRestored, domain.TransactionPurged:
		return OutcomeSuccess
	case domain.TransactionFailed:
		return OutcomeFailed
	default:
		return OutcomePartial
	}
}

// Envelope maps CleanupTransactionView onto the shared JSON envelope
// (issue #59). Only FAILED/SKIPPED items become issues -- MOVED/RESTORED/
// PURGED/PENDING items succeeded or simply haven't run yet, neither of
// which is a problem to surface.
func (v CleanupTransactionView) Envelope() EnvelopeOptions {
	opts := EnvelopeOptions{Outcome: transactionOutcome(v.Status)}
	for _, item := range v.Items {
		if item.Status != domain.TransactionItemFailed && item.Status != domain.TransactionItemSkipped {
			continue
		}
		severity := "WARNING"
		if item.Status == domain.TransactionItemFailed {
			severity = "ERROR"
		}
		opts.Issues = append(opts.Issues, EnvelopeIssue{
			Code: string(item.Status), Severity: severity, Path: item.OriginalPath, Message: item.Reason,
		})
	}
	return opts
}

func CleanupTransactionViewFromDomain(transaction domain.CleanupTransaction) CleanupTransactionView {
	view := CleanupTransactionView{ID: transaction.ID, PlanID: transaction.PlanID, Status: transaction.Status}
	for _, item := range transaction.Items {
		view.Items = append(view.Items, CleanupTransactionItemView{OriginalPath: item.OriginalPath, QuarantinePath: item.QuarantinePath, Status: item.Status, Reason: item.Reason})
	}
	return view
}
func (v CleanupTransactionView) RenderText(w io.Writer) error {
	fmt.Fprintf(w, "Transaction: %s\nStatus: %s\n", v.ID, v.Status)
	for _, item := range v.Items {
		fmt.Fprintf(w, "[%s] %s -> %s\n", item.Status, item.OriginalPath, item.QuarantinePath)
		if item.Reason != "" {
			fmt.Fprintf(w, "    %s\n", item.Reason)
		}
	}
	return nil
}
func (v CleanupTransactionsView) RenderText(w io.Writer) error {
	if len(v.Transactions) == 0 {
		_, err := fmt.Fprintln(w, "No cleanup transactions.")
		return err
	}
	for _, transaction := range v.Transactions {
		if err := transaction.RenderText(w); err != nil {
			return err
		}
		fmt.Fprintln(w)
	}
	return nil
}
