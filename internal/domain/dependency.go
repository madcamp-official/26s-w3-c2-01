package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// [파일 역할] Dependency는 PROJECT -> RESOURCE(현재는 REQUIRES 관계만) 유향
// 그래프 간선을 나타내는 domain 모델이다. DependencyID는 두 끝점과 관계로부터
// 안정적인 sha256 해시 ID를 만든다. evidence.go의 Evidence는 DependencyID
// 필드로 이 Dependency를 참조해 "왜 이 간선이 존재한다고 판단했는지" 근거를
// 덧붙이고, impact.go의 ImpactAssessment는 이 그래프를 바탕으로 리소스 제거
// 영향을 판정한다. internal/app/dependency_repository.go(DependencyRepository)가
// 저장 계약을 정의하며, internal/adapter/msbuild/resolve.go의
// ResolveDependencies가 실제로 이 그래프 간선을 만들어낸다.

type NodeType string

const (
	NodeProject  NodeType = "PROJECT"
	NodeResource NodeType = "RESOURCE"
)

type RelationType string

const (
	RelationRequires RelationType = "REQUIRES"
)

// Dependency is a directed graph edge. Day 4 creates PROJECT -> RESOURCE
// REQUIRES edges while retaining typed endpoints for future graph expansion.
type Dependency struct {
	ID         string
	SourceType NodeType
	SourceID   string
	TargetType NodeType
	TargetID   string
	Relation   RelationType
	Confidence int
}

// DependencyID returns the stable identity of one typed graph edge.
func DependencyID(sourceType NodeType, sourceID string, relation RelationType, targetType NodeType, targetID string) string {
	key := strings.Join([]string{string(sourceType), sourceID, string(relation), string(targetType), targetID}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}
