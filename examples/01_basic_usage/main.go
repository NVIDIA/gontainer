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
	"fmt"

	"github.com/NVIDIA/gontainer"
)

// MyService performs some crucial tasks.
type MyService struct{}

// SayHello outputs a friendly greeting.
func (s *MyService) SayHello(name string) {
	fmt.Println("Hello,", name)
}

func main() {
	// Initialize application container.
	// Order of factories definition is non-restrictive.
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
		panic("Failed to create service container: " + err.Error())
	}

	// Close defined services in reverse-to-instantiation order.
	// Every service can define it's own `Close() error` method.
	// The `container.Close()` can be called several times.
	defer func() {
		if err := container.Close(); err != nil {
			panic("Failed to close service container: " + err.Error())
		}
	}()

	// Instantiate all services within the container. Delayed Start() enables:
	// 1. Container validation without full service instantiation.
	// 2. Event dispatching to factories, e.g., to build help menus.
	if err := container.Start(); err != nil {
		panic("Failed to start service container: " + err.Error())
	}

	// At this point, all factories have been invoked.
	// Exiting here is fine for console scripts (see defer).
	// For daemons, it is OK to wait for `<-container.Done()`.
}
