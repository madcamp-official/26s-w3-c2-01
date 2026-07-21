package maven

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"os"
	"path/filepath"
	"strings"
)

type RepositoryLister struct {
	Environment cachepath.Environment
	ReadFile    func(string) ([]byte, error)
}

func (l RepositoryLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".m2", "repository")
	read := l.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	data, err := read(filepath.Join(home, ".m2", "settings.xml"))
	if err == nil {
		var settings struct {
			LocalRepository string `xml:"localRepository"`
		}
		if err := xml.Unmarshal(data, &settings); err != nil {
			return nil, fmt.Errorf("decode Maven settings.xml: %w", err)
		}
		if value := strings.TrimSpace(settings.LocalRepository); value != "" {
			path = strings.ReplaceAll(value, "${user.home}", home)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Maven local repository", "maven", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
