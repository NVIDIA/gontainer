/*
 * SPDX-FileCopyrightText: Copyright (c) 2003 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package httpsvr

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/NVIDIA/gontainer"
	confmod "github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/config"
)

// Server is an HTTP server service.
type Server struct {
	logger *slog.Logger
	svrmux *http.ServeMux
	server *http.Server
	config Config
}

// GetMux returns the HTTP serve mux.
func (s *Server) GetMux() *http.ServeMux {
	return s.server.Handler.(*http.ServeMux)
}

// Start serves HTTP in background.
// This function should be called manually when the app is ready to start.
// See the `services.app.WithAppEntryPoint()` factory for more details.
func (s *Server) Start() error {
	// Prepare TCP listener socket for the server.
	s.logger.Info("Starting HTTP server", "address", s.config.ListenAddr)
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		s.logger.Error("Failed to open TCP listener", "address", s.config.ListenAddr, "error", err)
		return fmt.Errorf("failed to open TCP listener on %s: %w", s.config.ListenAddr, err)
	}

	// Start serving HTTP requests.
	go func() {
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Failed to serve HTTP requests", "error", err)
		}
	}()

	return nil
}

// Close function will be automatically called on the container close.
// See: https://github.com/NVIDIA/gontainer?tab=readme-ov-file#services.
func (s *Server) Close() error {
	// Stop serving HTTP requests.
	s.logger.Info("Closing HTTP server")
	if err := s.server.Close(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Failed to close HTTP server", "error", err)
			return fmt.Errorf("failed to stop HTTP server: %w", err)
		}
	}

	return nil
}

// WithHTTPServer returns a factory for the HTTP server service.
func WithHTTPServer() *gontainer.Factory {
	return gontainer.NewFactory(
		func(logger *slog.Logger, confsvc *confmod.Config) (*Server, error) {
			// Prepare HTTP server config.
			config := Config{}
			if err := confsvc.Load(&config); err != nil {
				return nil, fmt.Errorf("failed to load HTTP server config: %w", err)
			}

			// Prepare HTTP serve mux.
			serverMux := http.NewServeMux()

			// Prepare logger for the service.
			logger = logger.With(slog.String("service", "http"))

			// Prepare HTTP server instance.
			return &Server{
				logger: logger,
				config: config,
				svrmux: serverMux,
				server: &http.Server{
					Addr:    config.ListenAddr,
					Handler: serverMux,
				},
			}, nil
		},
	)
}
