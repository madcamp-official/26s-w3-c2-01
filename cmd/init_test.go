package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
)

func TestInitJSONUsesSharedEnvelope(t *testing.T) {
	cfgPath = filepath.Join(t.TempDir(), ".libra.yaml")
	jsonOutput = true
	t.Cleanup(func() { cfgPath = ""; jsonOutput = false })

	out := &bytes.Buffer{}
	initCmd.SetOut(out)
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	var view output.InitView
	envelope, err := output.DecodeEnvelope(out.Bytes(), &view)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "init" || !view.ConfigCreated || view.ConfigPath != cfgPath {
		t.Fatalf("envelope/view = %#v/%#v", envelope, view)
	}
}
