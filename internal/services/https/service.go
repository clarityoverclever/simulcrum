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

package https

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
	fmt.Printf("[%s] initializing service\n", cfg.Handler.ServiceName)
	return &Service{
		state:  core.StatusStopped,
		config: cfg,
	}
}

func (s *Service) Name() string {
	return s.config.Handler.ServiceName
}

func (s *Service) Run(l net.Listener) error {
	if s.config.Enabled {
		if err := s.start(); err != nil {
			fmt.Printf("[%s] failed to start server: %v\n", s.config.Handler.ServiceName, err)
		}
	}

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
			fmt.Printf("[%s] control message decode error: %v\n", s.config.Handler.ServiceName, err)
		}
		return
	}

	var resp core.ControlResponse

	switch msg.Action {
	case core.ActionStart:
		if err := s.start(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: fmt.Sprintf("[%s] %v", s.config.Handler.ServiceName, err.Error())}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: fmt.Sprintf("[%s] server started\n", s.config.Handler.ServiceName)}
		}
	case core.ActionStop:
		if err := s.stop(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: fmt.Sprintf("[%s] %v", s.config.Handler.ServiceName, err.Error())}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: fmt.Sprintf("[%s] server stopped\n", s.config.Handler.ServiceName)}
		}
	case core.ActionStatus:
		resp = core.ControlResponse{Status: "ok", Message: fmt.Sprintf("[%s] server "+string(s.getState()), s.config.Handler.ServiceName)}
	case core.ActionRestart:
		if err := s.restart(); err != nil {
			resp = core.ControlResponse{Status: "error", Message: fmt.Sprintf("[%s] %v", s.config.Handler.ServiceName, err.Error())}
		} else {
			resp = core.ControlResponse{Status: "ok", Message: fmt.Sprintf("[%s] server restarted\n", s.config.Handler.ServiceName)}
		}
	case core.ActionUpdate:
		resp = core.ControlResponse{Status: "ok", Message: fmt.Sprintf("[%s] no update action for static service\n", s.config.Handler.ServiceName)}
	default:
		resp = core.ControlResponse{Status: "error", Message: fmt.Sprintf("[%s] unknown action\n", s.config.Handler.ServiceName)}
	}

	_ = enc.Encode(resp)
}

func (s *Service) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == core.StatusRunning {
		return fmt.Errorf("[%s] server already running\n", s.config.Handler.ServiceName)
	}

	var err error
	s.server, err = New(s.config)
	if err != nil {
		s.state = core.StatusError
		return fmt.Errorf("[%s] failed to create server: %w", s.config.Handler.ServiceName, err)
	}

	go func() {
		if err := s.server.Start(); err != nil {
			s.setState(core.StatusError)
			fmt.Printf("[%s] server error: %v\n", s.config.Handler.ServiceName, err)
		}
	}()

	s.state = core.StatusRunning
	fmt.Printf("[%s] server started\n", s.config.Handler.ServiceName)
	return nil
}

func (s *Service) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != core.StatusRunning {
		return fmt.Errorf("[%s] server not running", s.config.Handler.ServiceName)
	}

	if s.server != nil {
		if err := s.server.Stop(); err != nil {
			s.state = core.StatusError
			return fmt.Errorf("[%s] failed to stop server: %w", s.config.Handler.ServiceName, err)
		}
	}

	s.state = core.StatusStopped
	fmt.Printf("[%s] server stopped\n", s.config.Handler.ServiceName)
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
