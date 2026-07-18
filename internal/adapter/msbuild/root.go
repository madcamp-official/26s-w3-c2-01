package msbuild

import (
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// ProjectRoot derives a build project's root directory, display name, and
// drive from the path of a marker file (a .sln, .vcxproj, or .csproj). The
// root is simply the marker file's containing directory -- nested marker
// files (e.g. a .vcxproj referenced by a .sln elsewhere) each get their own
// root independently of one another.
func ProjectRoot(markerPath string) (root, name, drive string, err error) {
	abs, err := pathutil.Absolute(markerPath)
	if err != nil {
		return "", "", "", err
	}
	root = filepath.Dir(abs)
	base := filepath.Base(abs)
	name = strings.TrimSuffix(base, filepath.Ext(base))
	drive = filepath.VolumeName(abs)
	return root, name, drive, nil
}
