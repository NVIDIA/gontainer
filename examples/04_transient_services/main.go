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
	"log"
	"math/rand"

	"github.com/NVIDIA/gontainer"
)

func main() {
	// Execute service container.
	log.Println("Executing service container")
	err := gontainer.Run(
		// Root context for container.
		context.Background(),

		// Return a function that returns an int.
		gontainer.NewFactory(func() func() int {
			// From the container perspective this is a regular service.
			// Container factory returns this service as a singleton:
			// once on the eager container start or once on the lazy
			// service resolution via resolver or invoker components.
			return func() int {
				return rand.Int()
			}
		}),

		// This factory is called once on the container start.
		// Depend on the function returned by the previous factory.
		// This could be used to produce transient services.
		gontainer.NewFactory(func(funcFromFactory1 func() int) error {
			log.Printf("New value: %d", funcFromFactory1())
			log.Printf("New value: %d", funcFromFactory1())
			log.Printf("New value: %d", funcFromFactory1())
			return nil
		}),
	)

	// Check if service container run failed.
	if err != nil {
		log.Panicf("Service container failed: %s", err)
	}

	// Service container successfully executed.
	log.Println("Service container executed")
}
