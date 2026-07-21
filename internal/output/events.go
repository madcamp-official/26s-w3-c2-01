package output

import (
	"fmt"
	"github.com/madcamp-official/26s-w3-c2-01/internal/eventlog"
	"io"
	"text/tabwriter"
)

type EventsView struct {
	Events []eventlog.Event `json:"events"`
}

func (v EventsView) RenderText(w io.Writer) error {
	if len(v.Events) == 0 {
		_, err := fmt.Fprintln(w, "No daemon events found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tKIND\tPATH\tOLD PATH\tSIZE\tERROR")
	for _, event := range v.Events {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n", event.At.Format("2006-01-02T15:04:05Z07:00"), event.Kind, emptyDash(event.Path), emptyDash(event.OldPath), event.Size, emptyDash(event.Error))
	}
	return tw.Flush()
}
