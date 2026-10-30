// Package config loads fru-tester's own YAML config (not a test profile).
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Listen is host:port for the API. fru-lab reaches it from inside its
	// container, so the default binds every interface; ApiToken guards it.
	Listen   string `yaml:"listen"`
	ApiToken string `yaml:"apiToken"`
	Logger   Logger `yaml:"logger"`
	// SystemFile keeps the Status page's CPU allocation; a relative path
	// is relative to the config file's directory.
	SystemFile string `yaml:"systemFile"`
}

type Logger struct {
	Level string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{Listen: "0.0.0.0:9100", Logger: Logger{Level: "info"}}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.ApiToken == "" {
		return nil, fmt.Errorf("%s: apiToken must be set", path)
	}
	if cfg.SystemFile == "" {
		cfg.SystemFile = "tester-system.json"
	}
	if !filepath.IsAbs(cfg.SystemFile) {
		cfg.SystemFile = filepath.Join(filepath.Dir(path), cfg.SystemFile)
	}
	return cfg, nil
}
