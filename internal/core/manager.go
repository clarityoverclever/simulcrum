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

package core

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type Manager interface {
	Path(name string) string
	Create(name string) (net.Listener, error)
	Close(name string) error
}

type manager struct {
	baseDir string
}

func New(baseDir string) (*manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &manager{baseDir: baseDir}, nil
}

func (m *manager) Path(name string) string {
	return filepath.Join(m.baseDir, fmt.Sprintf("%s.sock", name))
}

func (m *manager) Create(name string) (net.Listener, error) {
	path := m.Path(name)
	_ = os.Remove(path)

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (m *manager) Close(name string) error {
	path := m.Path(name)
	return os.Remove(path)
}
