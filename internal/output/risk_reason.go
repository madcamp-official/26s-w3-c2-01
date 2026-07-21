package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func renderRiskReasons(w io.Writer, reasons []domain.RiskReason) {
	if len(reasons) == 0 {
		return
	}
	fmt.Fprintf(w, "Reason: %s\n", riskReasonMessages(reasons))
}

func renderRiskReasonsIndented(w io.Writer, reasons []domain.RiskReason) {
	if len(reasons) == 0 {
		return
	}
	fmt.Fprintf(w, "    Reason: %s\n", riskReasonMessages(reasons))
}

func riskReasonMessages(reasons []domain.RiskReason) string {
	messages := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		messages = append(messages, reason.Message)
	}
	return strings.Join(messages, "; ")
}
