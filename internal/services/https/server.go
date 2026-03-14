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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"simulacrum/internal/core/tlscert"
	"time"

	"simulacrum/internal/services/web"
)

type Server struct {
	cfg          Config
	Server       *http.Server
	certProvider tlscert.CertificateProvider
}

type Config struct {
	Enabled     bool
	BindAddress string
	Handler     web.HandlerConfig
	Tls         tlscert.TLSConfig
}

func New(cfg Config) (*Server, error) {
	var certProvider tlscert.CertificateProvider

	switch cfg.Tls.Mode {
	case "static":
		cert, err := tls.LoadX509KeyPair(cfg.Tls.Cert, cfg.Tls.Key)
		if err != nil {
			return nil, fmt.Errorf("[%s] failed to load TLS certificate: %w", cfg.Handler.ServiceName, err)
		}

		certProvider = &tlscert.StaticProvider{Certificate: &cert}
	default:
		return nil, fmt.Errorf("[%s] unsupported TLS mode: %s", cfg.Handler.ServiceName, cfg.Tls.Mode)
	}

	return &Server{
		cfg:          cfg,
		certProvider: certProvider,
	}, nil
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
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return s.certProvider.GetCertificate(hello.ServerName)
			},
		},
	}

	fmt.Printf("[%s] listening on %s\n", s.cfg.Handler.ServiceName, s.cfg.BindAddress)
	if err := s.Server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
