package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

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
