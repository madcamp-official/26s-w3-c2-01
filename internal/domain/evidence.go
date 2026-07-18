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
	EvidenceInferred EvidenceKind = "INFERRED"
	EvidenceUnknown  EvidenceKind = "UNKNOWN"
)

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
