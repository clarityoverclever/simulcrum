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
	"fmt"
	"net/http"
	"simulcrum/internal/logger"
	"time"
)

type Server struct {
	cfg    Config
	Server *http.Server
}

type Config struct {
	Enabled     bool
	BindAddress string
	LogHeaders  bool
}

func New(cfg Config) (*Server, error) {
	return &Server{cfg: cfg}, nil
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		host := r.Host

		s.logRequest(clientIP, host, r)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Simulcrum - DNAT Working</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f0f0f0; }
        .container { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #2c3e50; }
        .info { background: #e8f4f8; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .success { color: #27ae60; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Simulcrum HTTP Server</h1>
        <p class="success">DNAT is working!</p>
        <div class="info">
            <p><strong>Client IP:</strong> %s</p>
            <p><strong>Requested Host:</strong> %s</p>
            <p><strong>Path:</strong> %s</p>
            <p><strong>Method:</strong> %s</p>
        </div>
        <p>If you're seeing this page, your DNS query was spoofed and the traffic was redirected via DNAT to this analysis server.</p>
    </div>
</body>
</html>`, clientIP, host, r.URL.Path, r.Method)
	})

	s.Server = &http.Server{
		Addr:         s.cfg.BindAddress,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println("Starting HTTP server on", s.cfg.BindAddress)
	if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *Server) logRequest(clientIP string, host string, r *http.Request) {
	logger.Info("HTTP request received",
		"client", clientIP,
		"host", host,
		"path", r.URL.Path,
		"method", r.Method,
	)
	
	if !s.cfg.LogHeaders {
		return
	}

	// capture headers
	for header, values := range r.Header {
		for _, value := range values {
			logger.Info("HTTP header captured",
				"header", header,
				"value", value,
			)
		}
	}
}

func (s *Server) Stop() error {
	if s.Server != nil {
		fmt.Println("Stopping HTTP server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.Server.Shutdown(ctx); err != nil {
			fmt.Println("HTTP server shutdown failed:", err)
		}

		fmt.Println("HTTP server stopped")
	}
	return nil
}
