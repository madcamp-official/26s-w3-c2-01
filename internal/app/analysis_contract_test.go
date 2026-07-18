package app

import (
	"errors"
	"testing"
)

func TestIssueWrapsCause(t *testing.T) {
	cause := errors.New("permission denied")
	issue := Issue{
		Code: IssueAccessDenied, Phase: PhaseDiscoverFiles, Path: `D:\Projects`,
		Operation: "read", Severity: IssueWarning, Message: cause.Error(), Cause: cause,
	}
	if !errors.Is(issue, cause) {
		t.Fatal("Issue must wrap its cause")
	}
	if issue.Error() == "" {
		t.Fatal("Issue.Error() is empty")
	}
}

func TestAnalysisPhasesAreStable(t *testing.T) {
	want := []AnalysisPhase{
		PhaseDiscoverFiles, PhaseDiscoverProjects, PhaseDiscoverSystemResources,
		PhaseAnalyzeProjectSettings, PhaseResolveDependencies, PhaseClassifyArtifacts,
		PhaseCalculateRisk, PhasePersistResults, PhaseCompleted,
	}
	seen := make(map[AnalysisPhase]struct{}, len(want))
	for _, phase := range want {
		if phase == "" {
			t.Fatal("analysis phase must not be empty")
		}
		if _, exists := seen[phase]; exists {
			t.Fatalf("duplicate analysis phase %q", phase)
		}
		seen[phase] = struct{}{}
	}
}
