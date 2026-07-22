package domain

import "testing"

func TestEvidenceIDGolden(t *testing.T) {
	got := EvidenceID("dependency-1", EvidenceDeclared, ClaimRequiredDependency, EvidenceSupports, `D:\Game\Client.vcxproj`, "WindowsTargetPlatformVersion", "10.0", "10.0.22621.0")
	const want = "8e6bf7e4744e4581caa82687e30fd4d2ba31b70ae06cd90030b0d080796983fd"
	if got != want {
		t.Fatalf("EvidenceID() = %q, want %q", got, want)
	}
}

func TestEvidenceIDIgnoresCollectionTime(t *testing.T) {
	first := EvidenceID("dependency-1", EvidenceResolved, ClaimRequiredDependency, EvidenceSupports, "project.csproj", "TargetFramework", "net8.0", "8.0")
	second := EvidenceID("dependency-1", EvidenceResolved, ClaimRequiredDependency, EvidenceSupports, "project.csproj", "TargetFramework", "net8.0", "8.0")
	if first != second {
		t.Fatal("EvidenceID() changed for the same evidence content")
	}
}

func TestEvidenceIDDifferentiatesClaim(t *testing.T) {
	ownership := EvidenceID("dep", EvidenceObserved, ClaimProjectOwnership, EvidenceSupports, "manifest", "property", "raw", "resolved")
	tracked := EvidenceID("dep", EvidenceObserved, ClaimNoTrackedOriginals, EvidenceSupports, "manifest", "property", "raw", "resolved")
	if ownership == tracked {
		t.Fatal("EvidenceID() must include claim identity")
	}
}

func TestEvidenceIDDifferentiatesPolarity(t *testing.T) {
	supports := EvidenceID("dep", EvidenceObserved, ClaimProjectOwnership, EvidenceSupports, "manifest", "property", "raw", "resolved")
	contradicts := EvidenceID("dep", EvidenceObserved, ClaimProjectOwnership, EvidenceContradicts, "manifest", "property", "raw", "resolved")
	if supports == contradicts {
		t.Fatal("EvidenceID() must include polarity")
	}
}

func TestEvidenceIDCanonicalizesEmptyPolarityAsSupports(t *testing.T) {
	empty := EvidenceID("dep", EvidenceObserved, ClaimProjectOwnership, "", "manifest", "property", "raw", "resolved")
	supports := EvidenceID("dep", EvidenceObserved, ClaimProjectOwnership, EvidenceSupports, "manifest", "property", "raw", "resolved")
	if empty != supports {
		t.Fatal("EvidenceID() must treat legacy empty polarity as SUPPORTS")
	}
}
