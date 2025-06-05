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

	"github.com/NVIDIA/gontainer"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/app"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/config"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/httpsvr"
	"github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/logging"
)

// Example of the environment configuration.
func init() {
	_ = os.Setenv("LOG_FORMAT", "text")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("HTTP_LISTEN", "127.0.0.1:8080")
}

func main() {
	// Initialize the service container.
	container, err := gontainer.New(
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
		// This service factory starts serving request.
		// It is guaranteed to be the last called factory.
		app.WithAppEntryPoint(),
	)
	if err != nil {
		panicf("Failed to create service container: %s", err)
	}

	// Initialize the container deferred close.
	// Note: the `container.Close()` is also called from the `initCloseSignals()` func.
	// It is OK to call it twice and guarantees that the container will be closed even
	// if the `initCloseSignals()` fails or some else part of code will try to close it
	// or the final `<-container.Done()` blocking read will be removed from the code.
	defer func() {
		if err := container.Close(); err != nil {
			panicf("Failed to close service container: %s", err)
		}
	}()

	// Instantiate all services in the container.
	// This means: call all factory functions with the step of
	// reordering of factories based on dependencies graph.
	if err := container.Start(); err != nil {
		panicf("Failed to start service container: %s", err)
	}

	// Initialize closing of container by a signal.
	initCloseSignals(container, func(err error) {
		panicf("Failed to close service container: %s", err)
	})

	// Wait for a container close by a signal.
	<-container.Done()
}
