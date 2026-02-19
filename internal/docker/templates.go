package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type GameTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Game        string            `json:"game"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	Ports       []string          `json:"ports"`
	Env         map[string]string `json:"env"`
	Volumes     map[string]string `json:"volumes"`
	Memory      string            `json:"memory"`
	CPU         float64           `json:"cpu"`
	ConfigFields []ConfigField    `json:"config_fields"`
}

type ConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // text, number, select, toggle
	Default     string `json:"default"`
	Description string `json:"description"`
	Options     []string `json:"options,omitempty"`
	EnvVar      string `json:"env_var"`
}

func LoadTemplates(dir string) ([]GameTemplate, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob templates: %w", err)
	}

	var templates []GameTemplate
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", f, err)
		}
		var t GameTemplate
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", f, err)
		}
		templates = append(templates, t)
	}
	return templates, nil
}
