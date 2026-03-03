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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"simulacrum/internal/core"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("usage: simctl <service> <command> [args...]\n")
		os.Exit(1)
	}

	service := os.Args[1]
	action := os.Args[2]
	params := map[string]any{}

	if len(os.Args) > 3 {
		for _, arg := range os.Args[3:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("invalid argument format: %s\n", arg)
				os.Exit(1)
			}

			key := parts[0]
			value := parts[1]
			params[key] = value
		}
	}

	message := core.ControlMessage{
		Action:  core.ControlAction(action),
		Service: service,
		Params:  params,
	}

	sockMan, err := core.New("/tmp/simulacrum")
	if err != nil {
		fmt.Printf("Failed to create socket manager: %v\n", err)
		os.Exit(1)
	}

	path := sockMan.Path(service)
	conn, err := net.Dial("unix", path)
	if err != nil {
		fmt.Println("Failed to connect to socket:", err)
		os.Exit(1)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	if err := enc.Encode(message); err != nil {
		fmt.Println("Failed to send message:", err)
	}

	var response core.ControlResponse
	if err := dec.Decode(&response); err != nil {
		fmt.Println("Failed to receive response:", err)
	}

	fmt.Println(response)
}
