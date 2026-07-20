package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// [파일 역할] Evidence는 dependency.go의 Dependency 간선 하나를 뒷받침하는
// "스캔이 관찰한 사실 하나"(예: .csproj의 PackageReference 선언값, 실제로
// 리졸브된 값 등)를 나타낸다. DependencyID 필드로 Dependency와 다대일로
// 연결되며, EvidenceKind(DECLARED/RESOLVED/OBSERVED/INFERRED/UNKNOWN)로
// 그 근거가 "선언만 됐는지/실제로 리졸브됐는지/관찰됐는지/추론됐는지"를
// 구분한다. EvidenceID는 CollectedAt(관찰 시각)을 식별자 계산에서 일부러
// 제외해서, 같은 사실을 다시 관찰해도 새 행이 쌓이지 않고 기존 행이 갱신되게
// 한다.

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
