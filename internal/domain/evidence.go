package domain

import "time"

// EvidenceKind indicates how a project-resource dependency was established.
type EvidenceKind string

const (
	EvidenceDeclared EvidenceKind = "DECLARED" // declared directly in project config
	EvidenceResolved EvidenceKind = "RESOLVED" // resolved by a build tool
	EvidenceObserved EvidenceKind = "OBSERVED" // observed in actual use or by the daemon
	EvidenceInferred EvidenceKind = "INFERRED" // inferred from path, filename, or timing
	EvidenceUnknown  EvidenceKind = "UNKNOWN"  // insufficient information
)

// Evidence is a single piece of proof backing a Dependency edge.
type Evidence struct {
	ID         string
	Kind       EvidenceKind
	Source     string // e.g. "GameClient.vcxproj"
	Property   string // e.g. "WindowsTargetPlatformVersion"
	ObservedAt time.Time
}
