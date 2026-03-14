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
	"path/filepath"

	"gopkg.in/yaml.v2"
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
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_addr"`
}

type HttpsConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_addr"`
}

type CommonWebConfig struct {
	SpoofPayload bool  `yaml:"spoof_payload"`
	LogHeaders   bool  `yaml:"log_headers"`
	MaxBodyKb    int64 `yaml:"max_body_kb"`
}

type NtpConfig struct {
	Enabled     bool    `yaml:"enabled"`
	BindAddress string  `yaml:"bind_addr"`
	Multiplier  float64 `yaml:"multiplier"`
}

type TlsConfig struct {
	Mode string `yaml:"cert_mode"`
	Cert string `yaml:"cert_file"`
	Key  string `yaml:"key_file"`
}

type Config struct {
	DNS       DnsConfig       `yaml:"dns"`
	HTTP      HttpConfig      `yaml:"http"`
	HTTPS     HttpsConfig     `yaml:"https"`
	CommonWeb CommonWebConfig `yaml:"common_web"`
	NTP       NtpConfig       `yaml:"ntp"`
	TLS       TlsConfig       `yaml:"tls"`
}

func Load(path string) (*Config, error) {
	var err error
	cfg := &Config{}

	configDir := filepath.Dir(path)

	if err = os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create configuration directory: %w", err)
	}

	if _, err = os.Stat(path); os.IsNotExist(err) {
		//  TODO: add config file template from embedded resources
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}

		fmt.Println("creating empty config file: ", path)
		defer f.Close()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}
