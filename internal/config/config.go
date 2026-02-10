package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ForwardRule struct {
	Local  string `yaml:"local"`
	Remote string `yaml:"remote"`
}

type Config struct {
	Server  string        `yaml:"server"`
	User    string        `yaml:"user"`
	KeyFile string        `yaml:"key_file"`
	Forwards []ForwardRule `yaml:"forwards"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
