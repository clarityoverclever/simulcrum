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

package dns

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"simulacrum/internal/core"
	"sync"
)

type Service struct {
	mu     sync.Mutex
	state  core.Status
	server *Server
	config Config
}

func Init(cfg Config) *Service {
	fmt.Println("initializing dns")
	if cfg.Enabled {
		service := &Service{
			state:  core.StatusStopped,
			config: cfg,
		}
		err := service.start()
		if err != nil {
			fmt.Printf("[dns] failed to start: %v\n", err)
		}

		return service
	}

	return &Service{
		state:  core.StatusStopped,
		config: cfg,
	}
}

func (s *Service) Name() string {
	return "dns"
}

func (s *Service) Run(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go s.handleConnection(conn)
	}
}

func (s *Service) handleConnection(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var msg core.ControlMessage
	if err := dec.Decode(&msg); err != nil {
		if err != io.EOF {
			fmt.Printf("[dns] decode error: %v\n", err)
		}
		return
	}

	var resp core.ControlResponse

	switch msg.Action {
	case core.ActionStart:
		if err := s.start(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: err.Error()}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: "dns started"}
		}
	case core.ActionStop:
		if err := s.stop(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: err.Error()}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: "dns stopped"}
		}
	case core.ActionStatus:
		resp = core.ControlResponse{Status: "ok", Message: string(s.getState())}
	case core.ActionRestart:
		if err := s.restart(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: err.Error()}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: "dns restarted"}
		}
	case core.ActionUpdate:
		fmt.Println("dns updated")
		resp = core.ControlResponse{Status: "ok", Message: "nothing to update on static service"}
	default:
		resp = core.ControlResponse{Status: "error", Message: "unknown action"}
	}

	_ = enc.Encode(resp)
}

func (s *Service) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == core.StatusRunning {
		return fmt.Errorf("dns server already running")
	}

	var err error
	s.server, err = New(s.config)
	if err != nil {
		s.state = core.StatusError
		return fmt.Errorf("failed to create DNS server: %w", err)
	}

	// Start DNS server in a goroutine since it blocks
	go func() {
		if err := s.server.Start(); err != nil {
			s.setState(core.StatusError)
			fmt.Printf("[dns] server error: %v\n", err)
		}
	}()

	s.state = core.StatusRunning
	fmt.Println("dns started")
	return nil
}

func (s *Service) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != core.StatusRunning {
		return fmt.Errorf("dns server not running")
	}

	if s.server != nil {
		if err := s.server.Stop(); err != nil {
			s.state = core.StatusError
			return fmt.Errorf("failed to stop DNS server: %w", err)
		}
	}

	s.state = core.StatusStopped
	fmt.Println("dns stopped")
	return nil
}

func (s *Service) restart() error {
	if err := s.stop(); err != nil && s.getState() != core.StatusStopped {
		return err
	}
	return s.start()
}

func (s *Service) setState(state core.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *Service) getState() core.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
