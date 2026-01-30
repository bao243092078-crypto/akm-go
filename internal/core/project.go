package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents an akm.yaml project configuration file.
type ProjectConfig struct {
	Keys     []string `yaml:"keys"`
	Provider string   `yaml:"provider,omitempty"`
}

// LoadProjectConfig loads akm.yaml from the given directory.
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, "akm.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var config ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid akm.yaml: %w", err)
	}

	if len(config.Keys) == 0 {
		return nil, fmt.Errorf("akm.yaml has no keys defined")
	}

	return &config, nil
}

// FindProjectConfigs scans a parent directory for subdirectories containing akm.yaml.
func FindProjectConfigs(parentDir string) (map[string]*ProjectConfig, error) {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, err
	}

	configs := make(map[string]*ProjectConfig)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(parentDir, entry.Name())
		config, err := LoadProjectConfig(dir)
		if err != nil {
			continue // no akm.yaml or invalid, skip
		}
		configs[dir] = config
	}
	return configs, nil
}
