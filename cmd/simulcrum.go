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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"simulcrum/internal/config"
	"simulcrum/internal/dns"
	"simulcrum/internal/logger"
	"syscall"
)

func init() {}

func main() {
	// init config
	cfg, err := config.Load("./config/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "---CONFIG LOAD FAILURE---: %v\n", err)
		os.Exit(1)
	}

	// init logger
	if err := logger.Init(slog.LevelInfo, "./var/log/simulcrum/simulcrum.log"); err != nil {
		fmt.Fprintf(os.Stderr, "---LOGGER INIT FAILURE---: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("starting simulcrum", "version", "0.1.0")

	// capture and process terminating signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// abstract main into run to maintain logging while processing termination signals
	if err := run(cfg, quit); err != nil {
		fmt.Fprintf(os.Stderr, "---MAIN FAILURE---: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("simulcrum stopped")
}

// main application logic
func run(cfg *config.Config, quit <-chan os.Signal) error {
	// service initialization
	if cfg.DNS.Enabled {
		fmt.Println("starting DNS server")

		dnsServer, err := dns.New(dns.Config{
			Enabled:       cfg.DNS.Enabled,
			ListenAddr:    cfg.DNS.ListenAddr,
			AnalysisIP:    cfg.DNS.AnalysisIP,
			CheckLiveness: cfg.DNS.CheckLiveness,
			UpstreamDNS:   cfg.DNS.UpstreamDNS,
			SpoofNetwork:  cfg.DNS.SpoofNetwork,
			DefaultSubnet: cfg.DNS.DefaultSubnet,
		})

		if err != nil {
			return fmt.Errorf("failed to initialize DNS server: %w", err)
		}

		// start services
		errChan := make(chan error, 1)
		go func() {
			if err = dnsServer.Start(); err != nil {
				errChan <- fmt.Errorf("failed to start DNS server: %w", err)
			}
		}()

		select {
		case err = <-errChan:
			return fmt.Errorf("DNS server error: %w", err)
		case <-quit:
			fmt.Println("stopping services")
			if err = dnsServer.Stop(); err != nil {
				return fmt.Errorf("failed to stop DNS server: %w", err)
			}
		}
	} else {
		fmt.Println("DNS server not configured")
	}

	return nil
}
