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
