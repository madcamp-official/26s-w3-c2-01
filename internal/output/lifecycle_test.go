package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitViewRenderTextPlainWhenConfigAlreadyExists(t *testing.T) {
	out := &bytes.Buffer{}
	v := InitView{ConfigPath: "/tmp/.libra.yaml", DatabasePath: "/tmp/libra.db", ConfigCreated: false}
	if err := v.RenderText(out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("output = %q, re-running init on an existing config should never print the banner", out.String())
	}
	if !strings.Contains(out.String(), "Config file already exists") {
		t.Fatalf("output = %q, want the existing-config message", out.String())
	}
}

func TestInitViewRenderTextSkipsBannerWhenNotATerminal(t *testing.T) {
	out := &bytes.Buffer{}
	v := InitView{ConfigPath: "/tmp/.libra.yaml", DatabasePath: "/tmp/libra.db", ConfigCreated: true}
	if err := v.RenderText(out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("output = %q, piping init to a non-terminal writer must stay plain text", out.String())
	}
	if !strings.Contains(out.String(), "Created config file") {
		t.Fatalf("output = %q, want the created-config message", out.String())
	}
}

func TestBannerArtEmbedded(t *testing.T) {
	if !strings.Contains(bannerArt, "\x1b[") {
		t.Fatal("bannerArt should contain ANSI escape sequences from the embedded banner.ans")
	}
}
