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
	log.Println("Hello from the Hello Service", s.nameService.GetName())
}

func main() {
	// Initialize service container.
	log.Println("Running service container")
	err := gontainer.Run(
		// Root context for container.
		context.Background(),

		// Factory to create an instance of NameService.
		gontainer.NewFactory(func() *NameService {
			return &NameService{name: "Bob"}
		}),

		// Factory to create an instance of HelloService.
		gontainer.NewFactory(func(svc *NameService) *HelloService {
			return &HelloService{nameService: svc}
		}),

		// Function to use the HelloService instance.
		gontainer.NewFunction(func(svc *HelloService) {
			svc.SayHello()
		}),
	)
	if err != nil {
		log.Panicf("Failed to run service container: %s", err)
	}
	log.Println("Service container has run successfully")
}
