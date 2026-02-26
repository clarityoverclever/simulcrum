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

package dnat

import (
	"fmt"
	"os/exec"
	"simulcrum/internal/logger"
)

type Manager struct {
	analysisIP string
}

func New(analysisIP string) *Manager {
	return &Manager{analysisIP: analysisIP}
}

func (m *Manager) AddDNAT(spoofedIP string) error {
	logger.Info("adding DNAT rule",
		"spoofedIP", spoofedIP,
		"analysisIP", m.analysisIP,
	)
	cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-d", spoofedIP, "-j", "DNAT", "--to-destination", m.analysisIP)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("failed to add DNAT rule",
			"spoofedIP", spoofedIP,
			"analysisIP", m.analysisIP,
			"error", err,
			"output", string(output),
		)
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	return nil
}

func (m *Manager) RemoveDNAT(spoofedIP string) error {
	logger.Info("adding DNAT rule",
		"spoofedIP", spoofedIP,
		"analysisIP", m.analysisIP,
	)
	cmd := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING", "-d", spoofedIP, "-j", "DNAT", "--to-destination", m.analysisIP)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("failed to add DNAT rule",
			"spoofedIP", spoofedIP,
			"analysisIP", m.analysisIP,
			"error", err,
			"output", string(output),
		)
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	return nil
}

func (m *Manager) FlushAll() error {
	cmd := "iptables -t nat -F PREROUTING"
	return exec.Command("sh", "-c", cmd).Run()
}
