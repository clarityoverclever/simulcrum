// Copyright 2026 Keith Marshall
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DnsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	BindAddress   string `yaml:"bind_addr"`
	AnalysisIP    string `yaml:"analysis_ip"`
	CheckLiveness bool   `yaml:"check_liveness"`
	UpstreamDNS   string `yaml:"upstream_dns"`
	SpoofNetwork  bool   `yaml:"spoof_network"`
	DefaultSubnet string `yaml:"default_subnet"`
}

type HttpConfig struct {
	Enabled      bool   `yaml:"enabled"`
	BindAddress  string `yaml:"bind_addr"`
	LogHeaders   bool   `yaml:"log_headers"`
	SpoofPayload bool   `yaml:"spoof_payload"`
}

type Config struct {
	DNS  DnsConfig  `yaml:"dns"`
	HTTP HttpConfig `yaml:"http"`
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
