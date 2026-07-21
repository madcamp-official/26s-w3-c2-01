// Package docker reads aggregate Docker daemon disk usage through the
// official Docker CLI. It never invokes prune or any other mutating command.
package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type UsageLister interface {
	ListUsage(context.Context) ([]domain.Resource, error)
}

type CLILister struct {
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) ([]byte, error)
}

func (l CLILister) ListUsage(ctx context.Context) ([]domain.Resource, error) {
	lookPath := l.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	path, err := lookPath("docker")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("locate docker CLI: %w", err)
	}
	run := l.Run
	if run == nil {
		run = func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, executable, args...).Output()
		}
	}
	output, err := run(ctx, path, "system", "df", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker system df: %w", err)
	}
	return parseSystemDF(output, path)
}

type systemDFRow struct {
	Type        string `json:"Type"`
	Size        string `json:"Size"`
	Reclaimable string `json:"Reclaimable"`
}

func parseSystemDF(output []byte, dockerPath string) ([]domain.Resource, error) {
	var resources []domain.Resource
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var row systemDFRow
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("decode docker system df row: %w", err)
		}
		resourceType, slug, ok := classifyRow(row.Type)
		if !ok {
			continue
		}
		size, err := parseByteSize(row.Size)
		if err != nil {
			return nil, fmt.Errorf("decode Docker %s size: %w", row.Type, err)
		}
		reclaimableText := strings.Fields(row.Reclaimable)
		if len(reclaimableText) == 0 {
			return nil, fmt.Errorf("decode Docker %s reclaimable size: empty value", row.Type)
		}
		reclaimable, err := parseByteSize(reclaimableText[0])
		if err != nil {
			return nil, fmt.Errorf("decode Docker %s reclaimable size: %w", row.Type, err)
		}
		// Docker reports these as aggregate daemon-wide categories, not
		// distinct filesystem paths -- but resources.normalized_path is
		// UNIQUE per row, so each category needs its own synthetic path
		// under the CLI binary's, or the second category to upsert
		// collides with the first.
		resources = append(resources, domain.Resource{
			Name: "Docker " + row.Type, Type: resourceType, Version: slug,
			DisplayPath: filepath.Join(dockerPath, slug), LogicalSize: size, SizeKnown: true,
			ReclaimableSize: reclaimable,
			Confidence:      domain.DefaultConfidence[domain.EvidenceResolved],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return resources, nil
}

func classifyRow(value string) (domain.ResourceType, string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "images":
		return domain.ResourceTypeDockerCache, "images", true
	case "containers":
		return domain.ResourceTypeDockerCache, "containers", true
	case "build cache":
		return domain.ResourceTypeDockerCache, "build-cache", true
	case "local volumes":
		return domain.ResourceTypeDockerVolume, "local-volumes", true
	default:
		return "", "", false
	}
}

var byteSizePattern = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*([kmgt]?b)$`)

func parseByteSize(value string) (int64, error) {
	match := byteSizePattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, fmt.Errorf("unsupported byte size %q", value)
	}
	number, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	multipliers := map[string]float64{"b": 1, "kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12}
	return int64(number * multipliers[strings.ToLower(match[2])]), nil
}
