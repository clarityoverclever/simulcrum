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

package web

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"simulacrum/internal/services/hash"
	"simulacrum/internal/services/logger"
	"time"
)

//go:embed static/*
var staticDir embed.FS

type HandlerConfig struct {
	ServiceName  string
	SpoofPayload bool
	LogHeaders   bool
	MaxBodyKb    int64
}

type Handler struct {
	cfg HandlerConfig
}

var mimes = map[string]string{
	".exe": "application/x-msdownload",
	".dll": "application/x-msdownload",
	".ps1": "text/plain",
	".dat": "application/octet-stream",
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{cfg: cfg}
}

func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	h.LogRequest(r)

	switch r.Method {
	case "GET":
		h.HandleGet(w, r)
	case "POST":
		h.HandlePost(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if h.cfg.SpoofPayload {
		leaf := r.PathValue("path")
		ext := filepath.Ext(leaf) // extracts extension from the leaf path

		val, ok := mimes[ext]
		if ok {
			h.ServeFile(w, leaf, ext, val)
			return
		}
	}

	// default serve a 200
	w.Header().Set("Content-Type", "text/html")

	_, err := w.Write([]byte("<!DOCTYPE html><html><head><title>simulacrum</title></head><body bgcolor=\"grey\"><center><h1>OK</h1></center></body></html>"))
	if err != nil {
		logger.Error(fmt.Sprintf("[%s] failed to write response", h.cfg.ServiceName), "error", err)
	}
}

func (h *Handler) HandlePost(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[%s] POST received\n", h.cfg.ServiceName)

	var capture []byte
	var truncated bool
	var maxBodyBytes = h.cfg.MaxBodyKb * 1024 // convert to bytes for io.LimitReader

	limitReader := io.LimitReader(r.Body, maxBodyBytes+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		// logging the read error but processing any data collected for "best effort" result
		logger.Error(fmt.Sprintf("[%s] failed to read POST body", h.cfg.ServiceName), "error", err)
	}

	if int64(len(data)) > maxBodyBytes {
		truncated = true
		data = data[:maxBodyBytes]
	}

	capture = []byte(base64.StdEncoding.EncodeToString(data))
	preview, err := hash.GetXxHash(capture)
	if err != nil {
		logger.Error(fmt.Sprintf("[%s] failed to generate preview hash", h.cfg.ServiceName), "error", err)
	}

	logger.Info(fmt.Sprintf("[%s] POST data captured", h.cfg.ServiceName),
		"client", r.RemoteAddr,
		"host", r.Host,
		"path", r.URL.Path,
		"truncated", truncated,
		"bytes", len(capture),
		"preview", preview,
	)

	if len(capture) > 0 {
		err = h.CapturePostBody(preview, capture)
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to write capture to file", h.cfg.ServiceName), "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("OK"))
	if err != nil {
		logger.Error(fmt.Sprintf("[%s] failed to write response", h.cfg.ServiceName), "error", err)
	}
}

func (h *Handler) ServeFile(w http.ResponseWriter, fileName string, fileType string, contentType string) {
	logger.Info(fmt.Sprintf("[%s] Serving agent payload", h.cfg.ServiceName), "file_type", fileType)

	switch fileType {
	case ".exe":
		fmt.Printf("[%s] serving x64 payload\n", h.cfg.ServiceName)
		w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
		w.Header().Set("Content-Type", contentType)

		// serve x64 payload
		agent, err := staticDir.ReadFile("static/agent.exe")
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to read agent.exe", h.cfg.ServiceName), "error", err)
			return
		}
		_, err = w.Write(agent)
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to write agent.exe", h.cfg.ServiceName), "error", err)
			return
		}
	case ".ps1":
		fmt.Printf("[%s] serving powershell payload\n", h.cfg.ServiceName)
		w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
		w.Header().Set("Content-Type", contentType)

		// serve powershell payload
		agent, err := staticDir.ReadFile("static/agent.ps1")
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to read agent.ps1", h.cfg.ServiceName), "error", err)
			return
		}
		_, err = w.Write(agent)
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to write agent.ps1", h.cfg.ServiceName), "error", err)
			return
		}
	case ".dat", ".dll": // binary data
		w.Header().Set("Content-Type", contentType)

		// generate a payload with random binary data
		size := 1024*1024 + rand.Intn(4*1024*1024)
		_, err := io.CopyN(w, rand.New(rand.NewSource(time.Now().UnixNano())), int64(size))
		if err != nil {
			logger.Error(fmt.Sprintf("[%s] failed to write random data", h.cfg.ServiceName), "error", err)
		}
	default:
		return
	}
}

func (h *Handler) LogRequest(r *http.Request) {
	logger.Info(fmt.Sprintf("[%s] request received", h.cfg.ServiceName),
		"client", r.RemoteAddr,
		"host", r.Host,
		"path", r.URL.Path,
		"wildcard_path", r.PathValue("path"),
		"method", r.Method,
	)

	if !h.cfg.LogHeaders {
		return
	}

	// capture headers
	for header, values := range r.Header {
		for _, value := range values {
			logger.Info(fmt.Sprintf("[%s] header captured", h.cfg.ServiceName),
				"header", header,
				"value", value,
			)
		}
	}
}

func (h *Handler) CapturePostBody(file string, data []byte) error {
	var err error
	path := "./captures/"
	file = filepath.Join(path, file+".b64")

	fmt.Printf("[%s] writing capture to file %s\n", h.cfg.ServiceName, file)

	if err = os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create captures directory: %w", err)
	}

	if _, err = os.Stat(file); err == nil {
		logger.Info(fmt.Sprintf("[%s] duplicate POST capture skipped", h.cfg.ServiceName), "file", file)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("[%s] failed to stat capture file: %w", h.cfg.ServiceName, err)
	} else {
		err = os.WriteFile(file, data, 0644)
		if err != nil {
			return fmt.Errorf("[%s] failed to write capture to file: %w", h.cfg.ServiceName, err)
		}
	}
	return nil
}
