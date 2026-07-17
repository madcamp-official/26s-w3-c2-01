// Package scanner defines and implements bounded filesystem traversal for Libra.
package scanner

import (
	"context"
	"errors"
	"io/fs"
	"time"
)

// Scanner traverses configured roots. Recoverable path issues are returned in
// Result.Issues; error is reserved for failures that prevent the scan itself.
type Scanner interface {
	Scan(ctx context.Context, options Options, visit Visitor) (Result, error)
}

// Visitor consumes entries during traversal so large scans do not need to keep
// every path in memory. A nil visitor is valid.
type Visitor func(context.Context, Entry) error

type Options struct {
	Roots               []string
	Exclude             []string
	MaxDepth            int
	FollowReparsePoints bool
}

func (o Options) Validate() error {
	if len(o.Roots) == 0 {
		return errors.New("at least one scan root is required")
	}
	if o.MaxDepth <= 0 {
		return errors.New("max depth must be greater than zero")
	}
	return nil
}

type Entry struct {
	Path       string
	Kind       EntryKind
	Size       int64
	Mode       fs.FileMode
	ModifiedAt time.Time
}

type EntryKind string

const (
	EntryFile      EntryKind = "file"
	EntryDirectory EntryKind = "directory"
	EntrySymlink   EntryKind = "symlink"
	EntryOther     EntryKind = "other"
)

type Result struct {
	RootsScanned   int
	FilesInspected int64
	Directories    int64
	LogicalSize    int64
	Issues         []Issue
}

// Issue records a recoverable filesystem problem without stopping traversal.
type Issue struct {
	Path      string
	Operation string
	Err       error
}

func (i Issue) Error() string {
	if i.Path == "" {
		return i.Operation + ": " + i.Err.Error()
	}
	return i.Operation + " " + i.Path + ": " + i.Err.Error()
}

func (i Issue) Unwrap() error {
	return i.Err
}
