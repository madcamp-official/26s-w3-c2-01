package eventlog

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	want := Event{At: time.Now().UTC().Truncate(time.Nanosecond), Kind: "RESOURCE_DIRTY"}
	if err := Append(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || len(got) != 1 || got[0].Kind != want.Kind {
		t.Fatalf("Read = %#v, %v", got, err)
	}
}
