package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type EvidenceKind string

const (
	EvidenceDeclared EvidenceKind = "DECLARED"
	EvidenceResolved EvidenceKind = "RESOLVED"
	EvidenceObserved EvidenceKind = "OBSERVED"
	// EvidencePinned sits between DECLARED and INFERRED
	// (docs/libra_integration_contracts.md §19.4 결정 2): a requirements.txt
	// with every entry version-pinned ("==") is stronger evidence than an
	// unpinned one, but weaker than an actual lockfile (poetry.lock/
	// Pipfile.lock/uv.lock, which stay DECLARED like Node's lockfiles).
	EvidencePinned   EvidenceKind = "PINNED"
	EvidenceInferred EvidenceKind = "INFERRED"
	EvidenceUnknown  EvidenceKind = "UNKNOWN"
)

// DefaultConfidence is the CONFIRMED MVP score for each EvidenceKind
// (docs/libra_integration_contracts.md §20.2). It is the single shared scale
// every adapter's Confidence value must be drawn from -- adapter-local
// tables that don't reuse these numbers drift apart silently (this is what
// happened before §20.2 was confirmed: internal/adapter/node and
// internal/adapter/msbuild/artifacts.go each had their own placeholder
// scale, unrelated to internal/adapter/msbuild/resolve.go's).
//
// This governs the *base* score for a single piece of evidence only.
// Combining multiple Evidence for the same Dependency (limited additive
// credit for corroborating facts) and UnverifiedScope penalties are not
// implemented yet -- no resource today carries more than one Evidence, so
// there is nothing to combine in practice.
var DefaultConfidence = map[EvidenceKind]int{
	EvidenceResolved: 90,
	EvidenceObserved: 85,
	EvidenceDeclared: 75,
	EvidencePinned:   60,
	EvidenceInferred: 40,
	EvidenceUnknown:  10,
}

// Evidence is one scan-owned fact supporting a Dependency edge.
type Evidence struct {
	ID            string
	DependencyID  string
	Kind          EvidenceKind
	SourcePath    string
	Property      string
	RawValue      string
	ResolvedValue string
	CollectedAt   time.Time
}

// EvidenceID identifies the content of a fact. CollectedAt is intentionally
// excluded so observing the same fact again refreshes it instead of creating
// an unbounded duplicate row.
func EvidenceID(dependencyID string, kind EvidenceKind, sourcePath, property, rawValue, resolvedValue string) string {
	key := strings.Join([]string{dependencyID, string(kind), sourcePath, property, rawValue, resolvedValue}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}
