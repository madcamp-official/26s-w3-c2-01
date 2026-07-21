package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/madcamp-official/26s-w3-c2-01/internal/eventlog"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/spf13/cobra"
)

const (
	daemonPollInterval = 2 * time.Second
	daemonStaleAfter   = 10 * time.Minute
)

type daemonState struct {
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	Heartbeat  time.Time `json:"heartbeat"`
	Roots      []string  `json:"roots"`
	LastScanAt time.Time `json:"last_scan_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

type daemonSnapshot struct {
	Files  int64
	Bytes  int64
	Latest int64
}

var daemonCmd = &cobra.Command{Use: "daemon", Short: "Monitor configured roots and refresh the scan index"}
var daemonStartCmd = &cobra.Command{Use: "start", Short: "Start the background monitor", Args: cobra.NoArgs, RunE: startDaemon}
var daemonStatusCmd = &cobra.Command{Use: "status", Short: "Show background monitor status", Args: cobra.NoArgs, RunE: showDaemonStatus}
var daemonStopCmd = &cobra.Command{Use: "stop", Short: "Stop the background monitor", Args: cobra.NoArgs, RunE: stopDaemon}
var daemonRunCmd = &cobra.Command{Use: "run", Hidden: true, Args: cobra.NoArgs, RunE: runDaemon}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd, daemonStatusCmd, daemonStopCmd, daemonRunCmd)
}

func daemonStatePath() string {
	return filepath.Join(filepath.Dir(configFilePath()), ".libra-daemon.json")
}
func daemonEventPath() string {
	return filepath.Join(filepath.Dir(configFilePath()), ".libra-events.jsonl")
}

func startDaemon(cmd *cobra.Command, _ []string) error {
	if state, err := readDaemonState(); err == nil && daemonStateFresh(state, time.Now()) {
		return fmt.Errorf("daemon is already running with PID %d", state.PID)
	}
	loaded, err := config.Load(configFilePath())
	if err != nil {
		return fmt.Errorf("load daemon config: %w", err)
	}
	if len(loaded.ProjectRoots) == 0 {
		return errors.New("daemon requires at least one configured project root")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{}
	if cfgPath != "" {
		args = append(args, "--config", cfgPath)
	}
	args = append(args, "daemon", "run")
	process := exec.Command(executable, args...)
	process.Stdin = nil
	process.Stdout = io.Discard
	process.Stderr = io.Discard
	configureDaemonProcess(process)
	if err := process.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	pid := process.Process.Pid
	_ = process.Process.Release()
	now := time.Now().UTC()
	if err := writeDaemonState(daemonState{
		PID: pid, StartedAt: now, Heartbeat: now, Roots: append([]string(nil), loaded.ProjectRoots...),
	}); err != nil {
		return fmt.Errorf("record daemon state: %w", err)
	}
	return output.New(cmd.OutOrStdout(), jsonOutput, "daemon start").Print(output.DaemonActionView{Status: "starting", PID: pid})
}

func showDaemonStatus(cmd *cobra.Command, _ []string) error {
	state, err := readDaemonState()
	if os.IsNotExist(err) {
		return output.New(cmd.OutOrStdout(), jsonOutput, "daemon status").Print(output.DaemonStatusView{Status: "stopped"})
	}
	if err != nil {
		return err
	}
	status := "stale"
	if daemonStateFresh(state, time.Now()) {
		status = "running"
	}
	view := output.DaemonStatusView{Status: status, PID: state.PID, StartedAt: state.StartedAt, Heartbeat: state.Heartbeat, Roots: state.Roots, LastScanAt: state.LastScanAt, LastError: state.LastError}
	return output.New(cmd.OutOrStdout(), jsonOutput, "daemon status").Print(view)
}

func stopDaemon(cmd *cobra.Command, _ []string) error {
	state, err := readDaemonState()
	if os.IsNotExist(err) {
		return output.New(cmd.OutOrStdout(), jsonOutput, "daemon stop").Print(output.DaemonActionView{Status: "already_stopped"})
	}
	if err != nil {
		return err
	}
	if daemonStateFresh(state, time.Now()) {
		process, err := os.FindProcess(state.PID)
		if err == nil {
			err = process.Kill()
		}
		if err != nil {
			return fmt.Errorf("stop daemon PID %d: %w", state.PID, err)
		}
	}
	if err := os.Remove(daemonStatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return output.New(cmd.OutOrStdout(), jsonOutput, "daemon stop").Print(output.DaemonActionView{Status: "stopped"})
}

func runDaemon(_ *cobra.Command, _ []string) error {
	cfg := config.Default()
	loaded, err := config.Load(configFilePath())
	if err != nil {
		return fmt.Errorf("load daemon config: %w", err)
	}
	cfg = loaded
	if len(cfg.ProjectRoots) == 0 {
		return errors.New("daemon requires at least one configured project root")
	}
	now := time.Now().UTC()
	state := daemonState{PID: os.Getpid(), StartedAt: now, Heartbeat: now, Roots: append([]string(nil), cfg.ProjectRoots...)}
	if err := writeDaemonState(state); err != nil {
		return err
	}
	appendDaemonEvent(now, "DAEMON_STARTED", "")
	previous, err := snapshotRoots(cfg)
	if err != nil {
		state.LastError = err.Error()
	}
	ticker := time.NewTicker(daemonPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		current, snapshotErr := snapshotRoots(cfg)
		state.Heartbeat = time.Now().UTC()
		if snapshotErr != nil {
			state.LastError = snapshotErr.Error()
		} else if current != previous {
			state.LastError = runDaemonScan()
			state.LastScanAt = time.Now().UTC()
			previous = current
			appendDaemonEvent(state.LastScanAt, "RESOURCE_DIRTY", state.LastError)
		}
		if err := writeDaemonState(state); err != nil {
			return err
		}
	}
	return nil
}

func runDaemonScan() string {
	executable, err := os.Executable()
	if err != nil {
		return err.Error()
	}
	args := []string{}
	if cfgPath != "" {
		args = append(args, "--config", cfgPath)
	}
	args = append(args, "scan")
	command := exec.Command(executable, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return err.Error()
	}
	return ""
}

func snapshotRoots(cfg config.Config) (daemonSnapshot, error) {
	excluded := map[string]bool{
		strings.ToLower(defaultConfigFilename):            true,
		strings.ToLower(defaultDBFilename):                true,
		strings.ToLower(filepath.Base(daemonStatePath())): true,
		strings.ToLower(filepath.Base(daemonEventPath())): true,
	}
	for _, name := range cfg.Exclude {
		excluded[strings.ToLower(name)] = true
	}
	var snapshot daemonSnapshot
	for _, root := range cfg.ProjectRoots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && path != root && excluded[strings.ToLower(entry.Name())] {
				return filepath.SkipDir
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			snapshot.Files++
			if !entry.IsDir() {
				snapshot.Bytes += info.Size()
			}
			if modified := info.ModTime().UnixNano(); modified > snapshot.Latest {
				snapshot.Latest = modified
			}
			return nil
		})
		if err != nil {
			return snapshot, fmt.Errorf("snapshot root %q: %w", root, err)
		}
	}
	return snapshot, nil
}

func daemonStateFresh(state daemonState, now time.Time) bool {
	age := now.Sub(state.Heartbeat)
	return state.PID > 0 && age >= 0 && age <= daemonStaleAfter
}
func readDaemonState() (daemonState, error) {
	data, err := os.ReadFile(daemonStatePath())
	if err != nil {
		return daemonState{}, err
	}
	var state daemonState
	err = json.Unmarshal(data, &state)
	return state, err
}
func writeDaemonState(state daemonState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temporary := daemonStatePath() + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Remove(daemonStatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temporary, daemonStatePath())
}
func appendDaemonEvent(at time.Time, kind, eventErr string) {
	_ = eventlog.Append(daemonEventPath(), eventlog.Event{At: at, Kind: kind, Error: eventErr})
}
