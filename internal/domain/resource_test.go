package domain

import "testing"

func TestResourceIDGolden(t *testing.T) {
	got := ResourceID(ResourceTypeWindowsSDK, "10.0.22621.0", `c:\program files (x86)\windows kits\10`)
	const want = "66c0ec074943685e1c301bdd169f36439bc5ad36420df1449b8267f0cd927825"
	if got != want {
		t.Fatalf("ResourceID() = %q, want %q", got, want)
	}
}

func TestResourceIDSeparatesFields(t *testing.T) {
	first := ResourceID(ResourceType("ab"), "c", "d")
	second := ResourceID(ResourceType("a"), "bc", "d")
	if first == second {
		t.Fatal("ResourceID() collided for different key fields")
	}
}
