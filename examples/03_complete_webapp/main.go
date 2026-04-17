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

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/app"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/config"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/httpsvr"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/logging"
	"github.com/NVIDIA/gontainer/v2"
)

// Example of the environment configuration.
func init() {
	_ = os.Setenv("LOG_FORMAT", "text")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("HTTP_LISTEN", "127.0.0.1:8080")
}

func main() {
	// Prepare terminate signals channel.
	terminate := make(chan os.Signal)
	signal.Notify(terminate, syscall.SIGTERM, syscall.SIGINT)

	// Execute service container.
	err := gontainer.Run(
		// Enable config service.
		config.WithConfig(),

		// Enable logger service.
		logging.WithSlogLogger(),

		// Enable HTTP server factory.
		httpsvr.WithHTTPServer(),

		// Enable application endpoints factory.
		app.WithAppEndpoints(),

		// Enable health check endpoints factory.
		app.WithHealthEndpoints(),

		// Enable application entrypoint factory.
		app.WithAppEntryPoint(terminate),
	)

	// Check if service container run failed.
	if err != nil {
		panicf("Service container failed: %s", err)
	}
}
