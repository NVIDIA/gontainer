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

package app

import (
	"log/slog"
	"net/http"

	"github.com/NVIDIA/gontainer"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/httpsvr"
)

// WithAppEndpoints returns a factory which configures app endpoints.
func WithAppEndpoints() *gontainer.Factory {
	return gontainer.NewFactory(
		func(logger *slog.Logger, server *httpsvr.Server) {
			logger = logger.With("service", "app")
			logger.Info("Configuring app endpoints")
			server.GetMux().HandleFunc(
				"/", func(w http.ResponseWriter, r *http.Request) {
					logger.Info("Serving home page", "remote-addr", r.RemoteAddr)
					_, _ = w.Write([]byte("Hello, world!"))
				},
			)
		},
	)
}

// WithHealthEndpoints returns a factory which configures health check endpoints.
func WithHealthEndpoints() *gontainer.Factory {
	return gontainer.NewFactory(
		func(logger *slog.Logger, server *httpsvr.Server) {
			logger = logger.With("service", "app")
			logger.Info("Configuring health endpoints")
			server.GetMux().HandleFunc(
				"/health", func(w http.ResponseWriter, r *http.Request) {
					logger.Info("Serving health check", "remote-addr", r.RemoteAddr)
					_, _ = w.Write([]byte("Alive!"))
				},
			)
		},
	)
}

// WithAppEntryPoint returns a factory which performs final app start.
func WithAppEntryPoint() *gontainer.Factory {
	return gontainer.NewFactory(
		func(server *httpsvr.Server) error {
			// Start serving requests.
			return server.Start()
		},
	)
}
