package app

import (
	"sort"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// AssessClaim combines evidence without rewarding duplicates from the same
// source family. Contradiction and stale-source caps intentionally dominate
// corroboration bonuses.
func AssessClaim(claim domain.ClaimType, evidence []domain.Evidence, now time.Time) domain.ClaimAssessment {
	strongest := map[string]int{}
	ids := make([]string, 0, len(evidence))
	contradicted, stale := false, false
	for _, item := range evidence {
		if item.Claim != claim {
			continue
		}
		ids = append(ids, item.ID)
		if item.Polarity == domain.EvidenceContradicts {
			contradicted = true
			continue
		}
		family := item.SourceFamily
		if family == "" {
			family = item.SourcePath
		}
		if family == "" {
			family = item.ID
		}
		if score := domain.DefaultConfidence[item.Kind]; score > strongest[family] {
			strongest[family] = score
		}
		if item.ValidUntil != nil && now.After(*item.ValidUntil) {
			stale = true
		}
	}
	if len(strongest) == 0 && !contradicted {
		return domain.ClaimAssessment{Claim: claim, Status: domain.ConfidenceUnknown}
	}
	scores := make([]int, 0, len(strongest))
	for _, score := range strongest {
		scores = append(scores, score)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(scores)))
	score := 0
	if len(scores) > 0 {
		score = scores[0]
	}
	for range scores[1:] {
		bonus := (100 - score) / 4
		if bonus > 5 {
			bonus = 5
		}
		score += bonus
	}
	status := domain.ConfidenceKnown
	if contradicted {
		if score > 49 {
			score = 49
		}
		status = domain.ConfidenceConflicted
	}
	if stale {
		if score > 30 {
			score = 30
		}
		if status != domain.ConfidenceConflicted {
			status = domain.ConfidencePartial
		}
	}
	sort.Strings(ids)
	return domain.ClaimAssessment{Claim: claim, Score: score, Status: status, EvidenceIDs: ids}
}

// AssessAxis uses required-claim limiting semantics: one weak prerequisite
// limits the axis even when the remaining claims are strongly supported.
func AssessAxis(axis domain.ConfidenceAxis, required []domain.ClaimType, evidence []domain.Evidence, now time.Time) domain.ConfidenceAssessment {
	result := domain.ConfidenceAssessment{Axis: axis, Score: 100, Status: domain.ConfidenceKnown}
	for _, claim := range required {
		assessment := AssessClaim(claim, evidence, now)
		result.Claims = append(result.Claims, assessment)
		if assessment.Score < result.Score {
			result.Score, result.LimitingClaim = assessment.Score, claim
		}
		if assessment.Status == domain.ConfidenceConflicted {
			result.Status = domain.ConfidenceConflicted
		} else if assessment.Status == domain.ConfidenceUnknown && result.Status != domain.ConfidenceConflicted {
			result.Status = domain.ConfidenceUnknown
		} else if assessment.Status == domain.ConfidencePartial && result.Status == domain.ConfidenceKnown {
			result.Status = domain.ConfidencePartial
		}
	}
	if len(required) == 0 {
		result.Score, result.Status = 0, domain.ConfidenceUnknown
	}
	return result
}
