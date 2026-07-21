package output

import (
	"fmt"
	"io"
	"time"
)

type InitView struct {
	ConfigPath    string `json:"config_path"`
	DatabasePath  string `json:"database_path"`
	ConfigCreated bool   `json:"config_created"`
}

func (v InitView) RenderText(w io.Writer) error {
	if v.ConfigCreated {
		fmt.Fprintf(w, "Created config file: %s\n", v.ConfigPath)
	} else {
		fmt.Fprintf(w, "Config file already exists: %s\n", v.ConfigPath)
	}
	_, err := fmt.Fprintf(w, "Database ready: %s\n", v.DatabasePath)
	return err
}

type DaemonActionView struct {
	Status string `json:"status"`
	PID    int    `json:"pid,omitempty"`
}

func (v DaemonActionView) RenderText(w io.Writer) error {
	switch v.Status {
	case "starting":
		_, err := fmt.Fprintf(w, "Daemon starting (PID %d).\n", v.PID)
		return err
	case "already_stopped":
		_, err := fmt.Fprintln(w, "Daemon is already stopped.")
		return err
	default:
		_, err := fmt.Fprintln(w, "Daemon stopped.")
		return err
	}
}

type DaemonStatusView struct {
	Status     string    `json:"status"`
	PID        int       `json:"pid,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	Heartbeat  time.Time `json:"heartbeat,omitempty"`
	Roots      []string  `json:"roots,omitempty"`
	LastScanAt time.Time `json:"last_scan_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

func (v DaemonStatusView) RenderText(w io.Writer) error {
	if v.Status == "stopped" {
		_, err := fmt.Fprintln(w, "Daemon is stopped.")
		return err
	}
	fmt.Fprintf(w, "Daemon is %s (PID %d).\nLast heartbeat: %s\n", v.Status, v.PID, v.Heartbeat.Format(time.RFC3339))
	if !v.LastScanAt.IsZero() {
		fmt.Fprintf(w, "Last scan: %s\n", v.LastScanAt.Format(time.RFC3339))
	}
	if v.LastError != "" {
		fmt.Fprintf(w, "Last error: %s\n", v.LastError)
	}
	return nil
}
