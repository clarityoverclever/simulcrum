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
	"net/http"
	"time"

	"github.com/google/uuid"
)

func sendHeartbeat(taskID, stage string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", "http://verification.net", nil)
	req.Header.Set("X-Lab-TaskID", taskID)
	req.Header.Set("X-Lab-Stage", stage)
	req.Header.Set("X-Lab-Time", time.Now().Format("15:04:05"))

	_, err := client.Do(req)
	return err
}

func main() {
	taskID := uuid.New().String()

	for {
		err := sendHeartbeat(taskID, "[exe] heartbeat")
		if err != nil {
			return
		}
		time.Sleep(30 * time.Second)
	}
}
