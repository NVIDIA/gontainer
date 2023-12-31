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

// MyService performs some crucial tasks.
type MyService struct{}

// SayHello outputs a friendly greeting.
func (s *MyService) SayHello(name string) {
	log.Println("Hello,", name)
}

func main() {
	// Initialize service container.
	// Order of factories definition is non-restrictive.
	log.Println("Creating service container instance")
	container, err := gontainer.New(
		// Factory function to create an instance of MyService.
		gontainer.NewFactory(func() *MyService {
			return new(MyService)
		}),

		// General-purpose factory with access to all services.
		// Factories can spawn multiple services or none.
		// The last return argument can specify a factory error.
		gontainer.NewFactory(func(service *MyService) {
			service.SayHello("Username")
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

	// Instantiate all services within the container.
	// This call will wait until all factories returns.
	log.Println("Starting service container")
	if err := container.Start(); err != nil {
		log.Fatalf("Failed to start service container: %s", err)
	}

	// At this point, all factories have been invoked.
	// Exiting without wait is OK for console scripts.
	// For daemons, it is OK to wait for `<-container.Done()`.
	log.Println("Not awaiting service container done")
}
