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
	"net"
	"os"
	"os/signal"
	"simulacrum/internal/core"
	"simulacrum/internal/services/config"
	"simulacrum/internal/services/logger"
	"syscall"
)

func main() {
	// initialize configuration
	cfg, err := config.Load("./config/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "--- CONFIG LOAD FAILURE --- : %v\n", err)
		os.Exit(1)
	}

	// initialize logger
	if err = logger.Init(slog.LevelInfo, "./logs/simulacrum.log"); err != nil {
		fmt.Fprintf(os.Stderr, "--- LOGGER INIT FAILURE --- : %v\n", err)
		os.Exit(1)
	}

	fmt.Println("starting Simulacrum version: 0.0.1")

	// capture and process terminating signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// abstract main into run to maintain logging while processing termination signals
	if err := run(cfg, quit); err != nil {
		fmt.Fprintf(os.Stderr, "--- MAIN FAILURE --- : %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Simulacrum stopped")
}

func run(cfg *config.Config, quit <-chan os.Signal) error {
	var err error
	errChan := make(chan error, 1)

	fmt.Println("IPC starting")
	sockMan, err := core.New("/tmp/simulacrum")
	if err != nil {
		return err
	}

	defer sockMan.Close("/tmp/simulacrum")
	fmt.Println("IPC started")

	services := []core.Service{}

	for _, service := range services {
		listener, err := sockMan.Create(service.Name())
		if err != nil {
			return err
		}

		go func(s core.Service, listener net.Listener) {
			errChan <- s.Run(listener)
		}(service, listener)
	}

	// wait for termination signal
	select {
	case err = <-errChan:
		return err
	case <-quit:
		fmt.Println("Simulacrum terminating")
	}

	return nil
}
