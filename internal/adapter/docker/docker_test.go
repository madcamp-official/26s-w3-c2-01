package docker

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestCLIListerParsesDockerSystemDF(t *testing.T) {
	lister := CLILister{
		LookPath: func(string) (string, error) { return `/usr/bin/docker`, nil },
		Run: func(_ context.Context, path string, args ...string) ([]byte, error) {
			if path != `/usr/bin/docker` || len(args) != 4 || args[0] != "system" || args[1] != "df" {
				t.Fatalf("command = %s %v", path, args)
			}
			return []byte("{\"Type\":\"Images\",\"Size\":\"16.43MB\",\"Reclaimable\":\"11.63MB (70%)\"}\n" +
				"{\"Type\":\"Local Volumes\",\"Size\":\"36B\",\"Reclaimable\":\"0B (0%)\"}\n" +
				"{\"Type\":\"Build Cache\",\"Size\":\"1.5GB\",\"Reclaimable\":\"1GB\"}\n"), nil
		},
	}
	got, err := lister.ListUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].LogicalSize != 16_430_000 || got[0].ReclaimableSize != 11_630_000 {
		t.Fatalf("resources = %#v", got)
	}
	if got[1].Type != domain.ResourceTypeDockerVolume || got[2].Version != "build-cache" {
		t.Fatalf("resource classifications = %#v", got)
	}
}

func TestCLIListerTreatsMissingDockerAsEmpty(t *testing.T) {
	lister := CLILister{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	got, err := lister.ListUsage(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("ListUsage() = %#v, %v", got, err)
	}
}

func TestCLIListerReportsDaemonFailure(t *testing.T) {
	lister := CLILister{
		LookPath: func(string) (string, error) { return "docker", nil },
		Run:      func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("daemon unavailable") },
	}
	if _, err := lister.ListUsage(context.Background()); err == nil {
		t.Fatal("ListUsage() error = nil")
	}
}
