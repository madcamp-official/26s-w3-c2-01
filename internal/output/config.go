package output

import (
	"fmt"
	"io"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"go.yaml.in/yaml/v3"
)

type ConfigView struct {
	Path   string        `json:"path"`
	Config config.Config `json:"config"`
}

func (v ConfigView) RenderText(w io.Writer) error {
	data, err := yaml.Marshal(v.Config)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Config: %s\n%s", v.Path, data)
	return err
}

type ConfigValidationView struct {
	Path  string `json:"path"`
	Valid bool   `json:"valid"`
}

func (v ConfigValidationView) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "Config is valid: %s\n", v.Path)
	return err
}

type ConfigUpdateView struct {
	Path  string `json:"path"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (v ConfigUpdateView) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "Updated %s in %s\n", v.Key, v.Path)
	return err
}
