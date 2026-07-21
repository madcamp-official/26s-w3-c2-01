package cmd

import (
	"fmt"
	"github.com/madcamp-official/26s-w3-c2-01/internal/eventlog"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

var eventsKind, eventsSince string
var eventsLimit int
var eventsCmd = &cobra.Command{Use: "events", Short: "List daemon events", Args: cobra.NoArgs, RunE: runEvents}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.Flags().StringVar(&eventsKind, "kind", "", "filter by event kind")
	eventsCmd.Flags().StringVar(&eventsSince, "since", "", "show events since RFC3339 time or duration (for example 24h)")
	eventsCmd.Flags().IntVar(&eventsLimit, "limit", 50, "maximum number of newest events to show (0 for all)")
}
func runEvents(cmd *cobra.Command, _ []string) error {
	if eventsLimit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}
	since, err := parseEventsSince(eventsSince, time.Now().UTC())
	if err != nil {
		return err
	}
	events, err := eventlog.Read(daemonEventPath())
	if err != nil {
		return fmt.Errorf("read daemon events: %w", err)
	}
	filtered := make([]eventlog.Event, 0, len(events))
	for _, event := range events {
		if eventsKind != "" && !strings.EqualFold(event.Kind, eventsKind) {
			continue
		}
		if !since.IsZero() && event.At.Before(since) {
			continue
		}
		filtered = append(filtered, event)
	}
	if eventsLimit > 0 && len(filtered) > eventsLimit {
		filtered = filtered[len(filtered)-eventsLimit:]
	}
	return output.New(cmd.OutOrStdout(), jsonOutput).Print(output.EventsView{Events: filtered})
}
func parseEventsSince(value string, now time.Time) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration < 0 {
			return time.Time{}, fmt.Errorf("--since duration must be positive")
		}
		return now.Add(-duration), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse --since: use RFC3339 or a duration such as 24h")
	}
	return parsed, nil
}
