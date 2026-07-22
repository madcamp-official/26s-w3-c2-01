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

func TestConfigSetExcludeStillProtectsSafetyDirectories(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = "" })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}

	if err := setConfig(configSetCmd, []string{"exclude", `C:\Windows,node_modules`}); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`C:\Windows`, "node_modules", "$RECYCLE.BIN", "System Volume Information"}
	if len(loaded.Exclude) != len(want) {
		t.Fatalf("Exclude = %#v, want %#v", loaded.Exclude, want)
	}
	for i := range want {
		if loaded.Exclude[i] != want[i] {
			t.Fatalf("Exclude = %#v, want %#v", loaded.Exclude, want)
		}
	}
}

func TestConfigSetExcludeDeleteRemovesUserAddedEntryOnly(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = ""; configSetDelete = false })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := setConfig(configSetCmd, []string{"exclude", "node_modules,my-temp-dir"}); err != nil {
		t.Fatal(err)
	}

	configSetDelete = true
	if err := setConfig(configSetCmd, []string{"exclude", "my-temp-dir"}); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"node_modules", "$RECYCLE.BIN", "System Volume Information"}
	if len(loaded.Exclude) != len(want) {
		t.Fatalf("Exclude = %#v, want %#v", loaded.Exclude, want)
	}
	for i := range want {
		if loaded.Exclude[i] != want[i] {
			t.Fatalf("Exclude = %#v, want %#v", loaded.Exclude, want)
		}
	}
}

func TestConfigSetExcludeDeleteRejectsSafetyExcludeWithoutChangingFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = ""; configSetDelete = false })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)

	configSetDelete = true
	if err := setConfig(configSetCmd, []string{"exclude", "$RECYCLE.BIN"}); err == nil {
		t.Fatal("expected an error removing a protected exclude")
	}

	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Fatal("rejected delete changed config")
	}
}

func TestConfigSetExcludeDeleteRejectsUnknownEntry(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = ""; configSetDelete = false })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}

	configSetDelete = true
	if err := setConfig(configSetCmd, []string{"exclude", "not-in-the-list"}); err == nil {
		t.Fatal("expected an error removing an absent entry")
	}
}

func TestConfigSetDeleteRejectsNonExcludeKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	t.Cleanup(func() { cfgPath = ""; configSetDelete = false })
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}

	configSetDelete = true
	if err := setConfig(configSetCmd, []string{"scan.max_depth", "31"}); err == nil {
		t.Fatal("expected an error using --delete with a non-exclude key")
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
