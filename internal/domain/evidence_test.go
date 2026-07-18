package domain

import "testing"

func TestEvidenceIDGolden(t *testing.T) {
	got := EvidenceID("dependency-1", EvidenceDeclared, `D:\Game\Client.vcxproj`, "WindowsTargetPlatformVersion", "10.0", "10.0.22621.0")
	const want = "ccd3675e9edf60a0bcc41c32f35e742749e02e7871df2ae34023772f5e2b9a1e"
	if got != want {
		t.Fatalf("EvidenceID() = %q, want %q", got, want)
	}
}

func TestEvidenceIDIgnoresCollectionTime(t *testing.T) {
	first := EvidenceID("dependency-1", EvidenceResolved, "project.csproj", "TargetFramework", "net8.0", "8.0")
	second := EvidenceID("dependency-1", EvidenceResolved, "project.csproj", "TargetFramework", "net8.0", "8.0")
	if first != second {
		t.Fatal("EvidenceID() changed for the same evidence content")
	}
}
