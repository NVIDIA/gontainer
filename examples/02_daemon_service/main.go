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
	"os/signal"
	"syscall"

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
	// Prepare terminate signals channel.
	terminate := make(chan os.Signal)
	signal.Notify(terminate, syscall.SIGTERM, syscall.SIGINT)

	// Prepare external to container object.
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Execute service container.
	log.Println("Executing service container")
	err := gontainer.Run(
		// Root context for container.
		context.Background(),

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

			return &MyServer{
				&http.Server{
					Handler: handler,
				},
			}, nil
		}),

		// Factory to start serving HTTP requests and wait for termination.
		gontainer.NewFunction(func(logger *log.Logger, server *MyServer) error {
			logger.Println("Starting listening on: http://127.0.0.1:8080")
			socket, err := net.Listen("tcp", "127.0.0.1:8080")
			if err != nil {
				return err
			}

			// Prepare error channel.
			errsChan := make(chan error, 1)

			// Start serving HTTP requests.
			go func() {
				// Close error channel after server shutdown.
				defer close(errsChan)

				// Start serving HTTP requests on the socket.
				logger.Println("Starting serving HTTP requests")
				if err := server.server.Serve(socket); err != nil {
					if !errors.Is(err, http.ErrServerClosed) {
						logger.Printf("Error serving HTTP requests: %s", err)
					}
					errsChan <- err
				}
			}()

			// Wait for termination.
			select {
			case err := <-errsChan:
				logger.Printf("Exiting from serving with error: %s", err)
				return err
			case <-terminate:
				logger.Println("Exiting from serving by signal")
				return nil
			}
		}),
	)

	// Check if service container run failed.
	if err != nil {
		log.Panicf("Service container failed: %s", err)
	}

	// Service container successfully executed.
	log.Println("Service container executed")
}
