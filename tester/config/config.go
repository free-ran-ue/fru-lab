// Package config loads fru-tester's own YAML config (not a test profile).
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Listen is host:port for the API. fru-lab reaches it from inside its
	// container, so the default binds every interface; ApiToken guards it.
	Listen   string `yaml:"listen"`
	ApiToken string `yaml:"apiToken"`
	Logger   Logger `yaml:"logger"`
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
	return cfg, nil
}
