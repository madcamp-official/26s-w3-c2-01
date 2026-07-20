// Package scanner defines and implements bounded filesystem traversal for Libra.
package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// Scanner traverses configured roots. Recoverable path issues are returned in
// Result.Issues; error is reserved for failures that prevent the scan itself.
type Scanner interface {
	Scan(ctx context.Context, options Options, visit Visitor) (Result, error)
}

type ParallelScanner struct {
	workers int
	readDir func(string) ([]os.DirEntry, error)
	lstat   func(string) (os.FileInfo, error)
}

// New creates a bounded parallel scanner. Non-positive worker counts select a
// conservative default based on the available CPUs.
func New(workers int) *ParallelScanner {
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 32 {
			workers = 32
		}
	}
	return &ParallelScanner{
		workers: workers,
		readDir: os.ReadDir,
		lstat:   os.Lstat,
	}
}

type directoryTask struct {
	path  string
	depth int
}

type directoryResult struct {
	entries  []Entry
	children []directoryTask
	issues   []Issue
}

// Scan walks directories concurrently while visitor calls remain serialized.
// It never follows symbolic links or Windows reparse points.
func (s *ParallelScanner) Scan(ctx context.Context, options Options, visit Visitor) (Result, error) {
	if err := options.Validate(); err != nil {
		return Result{}, err
	}
	matcher, err := newExcludeMatcher(options.Roots, options.Exclude)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	queue := s.prepareRoots(options.Roots, matcher, &result)
	result.RootsScanned = len(queue)
	if len(queue) == 0 {
		return result, nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	jobs := make(chan directoryTask)
	results := make(chan directoryResult, s.workers)
	var workers sync.WaitGroup
	for range s.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				scanned := s.scanDirectory(task, options.MaxDepth, matcher)
				select {
				case results <- scanned:
				case <-workerCtx.Done():
					return
				}
			}
		}()
	}
	defer func() {
		cancel()
		close(jobs)
		workers.Wait()
	}()

	active := 0
	for len(queue) > 0 || active > 0 {
		var next directoryTask
		var dispatch chan<- directoryTask
		if len(queue) > 0 {
			next = queue[0]
			dispatch = jobs
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case dispatch <- next:
			queue = queue[1:]
			active++
		case scanned := <-results:
			active--
			result.Directories++
			result.Issues = append(result.Issues, scanned.issues...)
			queue = append(queue, scanned.children...)
			for _, entry := range scanned.entries {
				if entry.Kind == EntryFile || entry.Kind == EntryOther {
					result.FilesInspected++
					result.LogicalSize += entry.Size
				}
				if visit != nil {
					if err := visit(ctx, entry); err != nil {
						return result, err
					}
				}
			}
		}
	}

	return result, nil
}

func (s *ParallelScanner) prepareRoots(roots []string, matcher excludeMatcher, result *Result) []directoryTask {
	tasks := make([]directoryTask, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		identity, err := pathutil.Normalize(root)
		if err != nil {
			result.Issues = append(result.Issues, Issue{Path: root, Operation: "normalize root", Err: err})
			continue
		}
		displayPath, err := pathutil.Absolute(root)
		if err != nil {
			result.Issues = append(result.Issues, Issue{Path: root, Operation: "resolve root", Err: err})
			continue
		}
		if _, exists := seen[identity]; exists || matcher.Matches(displayPath) {
			continue
		}
		seen[identity] = struct{}{}

		info, err := s.lstat(displayPath)
		if err != nil {
			result.Issues = append(result.Issues, Issue{Path: displayPath, Operation: "inspect root", Err: err})
			continue
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			result.Issues = append(result.Issues, Issue{Path: displayPath, Operation: "inspect root", Err: errors.New("root is not a traversable directory")})
			continue
		}
		tasks = append(tasks, directoryTask{path: displayPath})
	}
	return tasks
}

func (s *ParallelScanner) scanDirectory(task directoryTask, maxDepth int, matcher excludeMatcher) directoryResult {
	children, err := s.readDir(task.path)
	if err != nil {
		return directoryResult{issues: []Issue{{Path: task.path, Operation: "read directory", Err: err}}}
	}

	result := directoryResult{}
	childDepth := task.depth + 1
	for _, child := range children {
		if childDepth > maxDepth {
			continue
		}
		path := filepath.Join(task.path, child.Name())
		if matcher.Matches(path) {
			continue
		}

		info, err := s.lstat(path)
		if err != nil {
			result.issues = append(result.issues, Issue{Path: path, Operation: "inspect entry", Err: err})
			continue
		}
		entry := entryFromInfo(path, info)
		result.entries = append(result.entries, entry)
		if entry.Kind == EntryDirectory && childDepth < maxDepth {
			result.children = append(result.children, directoryTask{path: path, depth: childDepth})
		}
	}
	return result
}

// IsLinkLike reports whether info describes a symlink or NTFS reparse point
// (junction, mount point) rather than a real file or directory -- the same
// test entryFromInfo uses below to classify an Entry as EntrySymlink.
// Adapters outside this package use it to decide whether a path is safe to
// treat as project-owned before including it in cleanup evidence.
func IsLinkLike(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info)
}

func entryFromInfo(path string, info os.FileInfo) Entry {
	kind := EntryOther
	size := logicalSize(info)
	switch {
	case IsLinkLike(info):
		kind = EntrySymlink
		size = 0
	case info.IsDir():
		kind = EntryDirectory
		size = 0
	case info.Mode().IsRegular():
		kind = EntryFile
	}
	return Entry{Path: path, Kind: kind, Size: size, Mode: info.Mode(), ModifiedAt: info.ModTime()}
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
