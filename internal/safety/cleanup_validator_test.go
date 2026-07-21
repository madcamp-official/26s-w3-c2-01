package safety

import "testing"

// TestIsAllowedArtifactName covers docs/libra_integration_contracts.md §19.4
// 결정 6: Python cache/venv directory names must be structurally eligible
// for cleanup, egg-info's variable-prefix name must match by suffix, and
// conda environment directories must never be allowlisted (결정 4 keeps
// conda out of the cleanup path regardless of basename).
func TestIsAllowedArtifactName(t *testing.T) {
	cases := map[string]bool{
		"node_modules":   true,
		"dist":           true,
		".venv":          true,
		"venv":           true,
		"env":            true,
		"__pycache__":    true,
		".pytest_cache":  true,
		".mypy_cache":    true,
		"mypkg.egg-info": true,
		"a.egg-info":     true,
		"envs":           false, // a conda local prefix env directory name
		"conda-env":      false,
		"random-folder":  false,
	}
	for name, want := range cases {
		if got := isAllowedArtifactName(name); got != want {
			t.Errorf("isAllowedArtifactName(%q) = %v, want %v", name, got, want)
		}
	}
}
