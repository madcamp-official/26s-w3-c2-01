package cachepath

import (
	"os"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type Environment struct {
	Home string
	Env  func(string) string
	Stat func(string) (os.FileInfo, error)
}

func (e Environment) Lookup(key string) string {
	if e.Env != nil {
		return e.Env(key)
	}
	return os.Getenv(key)
}
func (e Environment) UserHome() (string, error) {
	if e.Home != "" {
		return e.Home, nil
	}
	return os.UserHomeDir()
}
func (e Environment) Directory(path string) bool {
	stat := e.Stat
	if stat == nil {
		stat = os.Stat
	}
	info, err := stat(path)
	return err == nil && info.IsDir()
}
func Resource(name, version, path string, resourceType domain.ResourceType, confidence int) domain.Resource {
	return domain.Resource{Name: name, Version: version, Type: resourceType, DisplayPath: path, Confidence: confidence}
}
