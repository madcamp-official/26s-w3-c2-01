package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
)

func TestConfigShowValidateAndSet(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = "" })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	configShowCmd.SetOut(out)
	if err := showConfig(configShowCmd, nil); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("config show produced no output")
	}
	if err := setConfig(configSetCmd, []string{"scan.max_depth", "31"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(cfgPath)
	if err != nil || loaded.Scan.MaxDepth != 31 {
		t.Fatalf("config = %#v, err = %v", loaded, err)
	}
	if err := validateConfig(configValidateCmd, nil); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSetRejectsUnsupportedKeyWithoutChangingFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = "" })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)
	if err := setConfig(configSetCmd, []string{"cleanup.default_mode", "delete"}); err == nil {
		t.Fatal("expected unsupported key error")
	}
	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Fatal("invalid update changed config")
	}
}
