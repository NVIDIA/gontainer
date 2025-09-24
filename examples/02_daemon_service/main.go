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
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/NVIDIA/gontainer"
)

// MyServer wraps HTTP Server.
type MyServer struct {
	server *http.Server
}

// Serve starts serving requests on the socket.
func (s *MyServer) Serve(socket net.Listener) error {
	return s.server.Serve(socket)
}

// Close shuts down the server.
func (s *MyServer) Close() error {
	return s.server.Shutdown(context.Background())
}

func main() {
	// Prepare close signals channel.
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	// Prepare external to container object.
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Run the service container.
	log.Println("Running service container")
	err := gontainer.Run(
		// Root context for container.
		context.Background(),

		// Inject singleton object.
		gontainer.NewService(logger),

		// Provide MyServer factory.
		gontainer.NewFactory(func(logger *log.Logger) (*MyServer, error) {
			logger.Println("Creating new HTTP server with handlers")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Received http request: %v", r.URL.Path)
				_, _ = w.Write([]byte("Hello, Username"))
			})

			server := &http.Server{Handler: handler}
			return &MyServer{server}, nil
		}),

		gontainer.NewFunction(func(logger *log.Logger, server *MyServer) error {
			logger.Println("Opening listening socket on http://127.0.0.1:8080")
			socket, err := net.Listen("tcp", "127.0.0.1:8080")
			if err != nil {
				logger.Printf("Failed to open listening socket: %s", err)
				return fmt.Errorf("failed to open listening socket: %w", err)
			}

			go func() {
				logger.Println("Starting serving HTTP requests")
				if err := server.Serve(socket); err != nil {
					if !errors.Is(err, http.ErrServerClosed) {
						logger.Printf("Error serving HTTP requests: %s", err)
					}
				}
			}()

			// Wait for the close signal.
			<-signals

			logger.Println("Close signal received, exiting")
			return nil
		}),
	)
	if err != nil {
		log.Panicf("Failed to run service container: %s", err)
	}

	log.Println("Service container has run successfully")
}
