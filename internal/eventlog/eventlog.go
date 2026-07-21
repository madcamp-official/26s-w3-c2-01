package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Event struct {
	At    time.Time `json:"at"`
	Kind  string    `json:"kind"`
	Error string    `json:"error,omitempty"`
}

func Append(path string, event Event) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(event)
}

func Read(path string) ([]Event, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode event log line %d: %w", line, err)
		}
		if event.At.IsZero() || event.Kind == "" {
			return nil, fmt.Errorf("decode event log line %d: at and kind are required", line)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
