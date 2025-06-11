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

func main() {
	// Initialize service container.
	// Order of factories definition is non-restrictive.
	log.Println("Creating new service container")
	container, err := gontainer.New(
		// Return a function that returns an int.
		gontainer.NewFactory(func() func() int {
			// From the container perspective this is a regular service.
			// Container factory returns this service as a singleton:
			// once on the eager container start or once on the lazy
			// service resolution via resolver or invoker components.
			return func() int { return 42 }
		}),

		// This factory is called once on the container start.
		// Depend on the function returned by the previous factory.
		// This could be used to produce transient services.
		gontainer.NewFactory(func(funcFromFactory1 func() int) {
			log.Printf("New value: %d", funcFromFactory1())
			log.Printf("New value: %d", funcFromFactory1())
			log.Printf("New value: %d", funcFromFactory1())
		}),
	)

	// Validate the container's proper handling of all factory functions.
	// Errors may point to bad function signatures or unresolvable dependencies.
	if err != nil {
		log.Panicf("Failed to create service container: %s", err)
	}

	// Instantiate all services within the container.
	// This call will wait until all factories returns.
	log.Println("Starting service container")
	if err := container.Start(); err != nil {
		log.Panicf("Failed to start service container: %s", err)
	}

	// Close defined services in reverse-to-instantiation order.
	// Every service can define it's own `Close() error` method.
	// The `container.Close()` can be called several times.
	defer func() {
		log.Println("Closing service container by defer")
		if err := container.Close(); err != nil {
			log.Panicf("Failed to close service container: %s", err)
		}
		log.Println("Service container closed")
	}()
}
