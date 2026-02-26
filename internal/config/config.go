package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DnsConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	DefaultIP  string `yaml:"default_ip"`
}
type Config struct {
	DNS DnsConfig `yaml:"dns"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}
