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
	"log"

	"github.com/NVIDIA/gontainer"
)

// NameService is a service that returns a name.
type NameService struct {
	name string
}

// GetName returns a name.
func (s *NameService) GetName() string {
	return "Bob"
}

// HelloService is a service that says hello.
type HelloService struct {
	nameService *NameService
}

// SayHello says hello to the name.
func (s *HelloService) SayHello() {
	log.Println("Hello,", s.nameService.GetName())
}

func main() {
	// Initialize service container.
	// Order of factories definition is non-restrictive.
	log.Println("Creating new service container")
	container, err := gontainer.New(
		// Factory to create an instance of NameService.
		gontainer.NewFactory(func() *NameService {
			return &NameService{name: "Bob"}
		}),

		// Factory to create an instance of HelloService.
		gontainer.NewFactory(func(svc *NameService) *HelloService {
			return &HelloService{nameService: svc}
		}),
	)

	// Validate the container's proper handling of all factory functions.
	// Errors may point to bad function signatures or unresolvable dependencies.
	if err != nil {
		log.Fatalf("Failed to create service container: %s", err)
	}

	// Close defined services in reverse-to-instantiation order.
	// Every service can define it's own `Close() error` method.
	// The `container.Close()` can be called several times.
	defer func() {
		log.Println("Closing service container by defer")
		if err := container.Close(); err != nil {
			log.Fatalf("Failed to close service container: %s", err)
		}
	}()

	// Application entrypoint using the `Invoker` service.
	// Invoker will resolve all dependencies for the function.
	_, err = container.Invoker().Invoke(func(svc *HelloService) {
		// Here the application bootstrap code could be located.
		// It can access all services from the container.
		// For example, HTTP server could be started here.
		svc.SayHello()
	})
	if err != nil {
		log.Panicf("Failed to invoke: %s", err)
	}

	// - or -

	// Application entrypoint using the `Resolver` service.
	// This code will manually resolve all dependencies.
	var helloService *HelloService
	err = container.Resolver().Resolve(&helloService)
	if err != nil {
		log.Panicf("Failed to resolve: %s", err)
	}

	// Manually call the service code.
	helloService.SayHello()
}
