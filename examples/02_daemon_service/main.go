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
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/NVIDIA/gontainer"
)

// MyService performs some crucial tasks.
type MyService struct{}

// SayHello outputs a friendly greeting.
func (s *MyService) SayHello(name string) string {
	return "Hello, " + name
}

// MyServer wraps HTTP Server.
type MyServer struct {
	server *http.Server
}

// Close implements close interface.
func (s *MyServer) Close() error {
	return s.server.Shutdown(context.Background())
}

func main() {
	// Prepare external to container object.
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Initialize service container.
	// Order of factories definition is non-restrictive.
	log.Println("Creating service container instance")
	container, err := gontainer.New(
		// Inject singleton object.
		gontainer.NewService(logger),

		// Factory function to create an instance of MyService.
		gontainer.NewFactory(func() *MyService {
			return new(MyService)
		}),

		// Factory function to create an instance of MyServer.
		gontainer.NewFactory(func(logger *log.Logger, svc *MyService) (*MyServer, error) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Received http request: %v", r.URL.Path)
				_, _ = w.Write([]byte(svc.SayHello("Username")))
			})

			logger.Println("Starting listening on: http://127.0.0.1:8080")
			socket, err := net.Listen("tcp", "127.0.0.1:8080")
			if err != nil {
				return nil, err
			}

			logger.Println("Starting serving HTTP requests")
			server := &http.Server{Handler: handler}
			go func() {
				if err := server.Serve(socket); err != nil {
					if !errors.Is(err, http.ErrServerClosed) {
						logger.Printf("Error serving HTTP requests: %s", err)
					}
				}
			}()

			return &MyServer{server}, nil
		}),
	)

	// Validate the container's proper handling of all factory functions.
	// Errors may point to bad function signatures or unresolvable dependencies.
	if err != nil {
		log.Panicf("Failed to create service container: %s", err)
	}

	// Close defined services in reverse-to-instantiation order.
	// Every service can define it's own `Close() error` method.
	// The `container.Close()` can be called several times.
	defer func() {
		log.Println("Closing service container by defer")
		if err := container.Close(); err != nil {
			log.Panicf("Failed to close service container: %s", err)
		}
	}()

	// Instantiate all services within the container.
	// This call will wait until all factories returns.
	log.Println("Starting service container")
	if err := container.Start(); err != nil {
		log.Panicf("Failed to start service container: %s", err)
	}

	// Initialize closing of container by a signal.
	initCloseSignals(container, func(err error) {
		log.Panicf("Failed to close service container: %s", err)
	})

	// Wait for close by signal.
	log.Println("Awaiting service container done")
	<-container.Done()
}
