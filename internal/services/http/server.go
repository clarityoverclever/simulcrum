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

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"simulacrum/internal/services/web"
)

type Server struct {
	cfg    Config
	Server *http.Server
}

type Config struct {
	Enabled     bool
	BindAddress string
	Handler     web.HandlerConfig
}

func New(cfg Config) (*Server, error) {
	return &Server{cfg: cfg}, nil
}

func (s *Server) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	handler := web.NewHandler(s.cfg.Handler)

	mux := http.NewServeMux()

	mux.HandleFunc("/{path...}", handler.HandleRequest)

	s.Server = &http.Server{
		Addr:         s.cfg.BindAddress,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("[%s] listening on %s\n", s.cfg.Handler.ServiceName, s.cfg.BindAddress)
	if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("[%s] failed to start server: %w", s.cfg.Handler.ServiceName, err)
	}

	return nil
}

func (s *Server) Stop() error {
	if s.Server != nil {
		fmt.Printf("[%s] Stopping server\n", s.cfg.Handler.ServiceName)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.Server.Shutdown(ctx); err != nil {
			return fmt.Errorf("[%s] failed to stop server: %w", s.cfg.Handler.ServiceName, err)
		}
	}

	return nil
}
