package python

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectMarkers_Priority(t *testing.T) {
	root := t.TempDir()
	write(t, root, "requirements.txt", "flask==2.0.0\n")
	write(t, root, "setup.py", "")
	write(t, root, "Pipfile", "")
	write(t, root, "pyproject.toml", "")

	markers, err := DetectMarkers(root)
	if err != nil {
		t.Fatal(err)
	}
	if markers.Primary != markerPyproject {
		t.Errorf("Primary = %q, want %q", markers.Primary, markerPyproject)
	}
	if len(markers.Secondary) != 3 {
		t.Errorf("Secondary = %v, want 3 entries", markers.Secondary)
	}
}

func TestDetectMarkers_RequirementsOnlyNeedsPythonFile(t *testing.T) {
	t.Run("no .py file: not a project", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "requirements.txt", "flask==2.0.0\n")
		markers, err := DetectMarkers(root)
		if err != nil {
			t.Fatal(err)
		}
		if markers.Primary != "" {
			t.Errorf("Primary = %q, want empty (no .py file gate)", markers.Primary)
		}
	})

	t.Run("with .py file: accepted", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "requirements.txt", "flask==2.0.0\n")
		write(t, root, "main.py", "print('hi')\n")
		markers, err := DetectMarkers(root)
		if err != nil {
			t.Fatal(err)
		}
		if markers.Primary != markerRequirements {
			t.Errorf("Primary = %q, want %q", markers.Primary, markerRequirements)
		}
	})
}

func TestFilesystemDetector_CanDetect(t *testing.T) {
	var detector Detector = FilesystemDetector{}

	withPyproject := t.TempDir()
	write(t, withPyproject, "pyproject.toml", "[project]\nname=\"x\"\n")

	empty := t.TempDir()

	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"pyproject.toml present", withPyproject, true},
		{"no markers", empty, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detector.CanDetect(scanner.Entry{Path: tc.dir}); got != tc.want {
				t.Errorf("CanDetect(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}

func TestFilesystemDetector_Detect(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pyproject.toml", "[project]\nname=\"x\"\n")

	var detector Detector = FilesystemDetector{}
	project, err := detector.Detect(context.Background(), scanner.Entry{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if project.Type != domain.ProjectTypePython {
		t.Errorf("Type = %q, want %q", project.Type, domain.ProjectTypePython)
	}
	if filepath.Base(project.ManifestPath) != "pyproject.toml" {
		t.Errorf("ManifestPath = %q, want to end in pyproject.toml", project.ManifestPath)
	}
}

func TestLockfileEvidence(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(root string)
		wantKind domain.EvidenceKind
	}{
		{"poetry.lock present", func(root string) { write(t, root, "poetry.lock", "") }, domain.EvidenceDeclared},
		{"fully pinned requirements.txt", func(root string) {
			write(t, root, "requirements.txt", "flask==2.0.0\nrequests==2.31.0\n# comment\n\n-r base.txt\n")
		}, domain.EvidencePinned},
		{"unpinned requirements.txt", func(root string) {
			write(t, root, "requirements.txt", "flask>=2.0.0\n")
		}, domain.EvidenceInferred},
		{"pyproject.toml only, no lock", func(root string) {
			write(t, root, "pyproject.toml", "[project]\nname=\"x\"\n")
		}, domain.EvidenceInferred},
		{"nothing at all", func(root string) {}, domain.EvidenceUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(root)
			kind, _ := LockfileEvidence(root)
			if kind != tc.wantKind {
				t.Errorf("LockfileEvidence kind = %q, want %q", kind, tc.wantKind)
			}
		})
	}
}

func TestDetectVenv_RequiresPyvenvCfg(t *testing.T) {
	t.Run("name match without pyvenv.cfg: not confirmed", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "env"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, ok, err := DetectVenv(root)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("DetectVenv should not confirm a directory without pyvenv.cfg")
		}
	})

	t.Run("pyvenv.cfg present: confirmed", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, filepath.Join(".venv", "pyvenv.cfg"), "home = /usr/bin\n")
		name, ok, err := DetectVenv(root)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || name != ".venv" {
			t.Errorf("DetectVenv = (%q, %v), want (.venv, true)", name, ok)
		}
	})
}

func TestDetectArtifacts(t *testing.T) {
	root := t.TempDir()
	write(t, root, "poetry.lock", "")
	write(t, root, filepath.Join(".venv", "pyvenv.cfg"), "home = /usr/bin\n")
	if err := os.MkdirAll(filepath.Join(root, "__pycache__"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "mypkg.egg-info"), 0o755); err != nil {
		t.Fatal(err)
	}

	resources, err := DetectArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}

	byName := make(map[string]domain.Resource)
	for _, r := range resources {
		byName[r.Name] = r
	}

	venv, ok := byName[".venv"]
	if !ok {
		t.Fatal("expected .venv resource")
	}
	if venv.Type != domain.ResourceTypeVenv || !venv.Regenerable || venv.RegenerationCommand != "poetry install" {
		t.Errorf(".venv resource = %+v, want Regenerable=true RegenerationCommand=poetry install", venv)
	}

	cache, ok := byName["__pycache__"]
	if !ok || cache.Type != domain.ResourceTypeBuildOutput || !cache.Regenerable {
		t.Errorf("__pycache__ resource = %+v, want BuildOutput/Regenerable", cache)
	}

	egg, ok := byName["mypkg.egg-info"]
	if !ok || egg.Type != domain.ResourceTypeBuildOutput || !egg.Regenerable {
		t.Errorf("mypkg.egg-info resource = %+v, want BuildOutput/Regenerable", egg)
	}
}

func TestDetectArtifacts_UnpinnedVenvNotRegenerable(t *testing.T) {
	root := t.TempDir()
	write(t, root, "requirements.txt", "flask>=2.0.0\n")
	write(t, root, filepath.Join("venv", "pyvenv.cfg"), "home = /usr/bin\n")

	resources, err := DetectArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range resources {
		if r.Name == "venv" {
			if r.Regenerable {
				t.Error("unpinned requirements.txt should leave venv Regenerable=false (결정 6)")
			}
			return
		}
	}
	t.Fatal("expected venv resource")
}
