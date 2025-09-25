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

package gontainer

import (
	"context"
	"fmt"
)

// Run runs a container with a set of configured factories.
func Run(ctx context.Context, factories ...*Factory) error {
	// Prepare container context ignoring the cancelling.
	// When cancelled, it closes `container.Done()` channel
	// and unblocks any waiting read from `container.Done()`.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	// Prepare services registry instance.
	registry := &registry{}

	// Prepare service resolver instance.
	resolver := &Resolver{registry: registry}

	// Prepare function invoker instance.
	invoker := &Invoker{resolver: resolver}

	// Register service resolver instance in the registry.
	if factory, err := NewService(resolver).factory(ctx); err != nil {
		return fmt.Errorf("failed to register service resolver: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register function invoker instance in the registry.
	if factory, err := NewService(invoker).factory(ctx); err != nil {
		return fmt.Errorf("failed to register function invoker: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register provided factories in the registry.
	for _, source := range factories {
		if factory, err := source.factory(ctx); err != nil {
			return fmt.Errorf("failed to register factory: %w", err)
		} else {
			registry.registerFactory(factory)
		}
	}

	// Validate all factories in the registry.
	if err := registry.validateFactories(); err != nil {
		return fmt.Errorf("failed to validate factories: %w", err)
	}

	// Start all factories in the container.
	if err := registry.spawnFactories(); err != nil {
		return fmt.Errorf("failed to spawn factories: %w", err)
	}

	// Close all factories in the container.
	if err := registry.closeFactories(); err != nil {
		return fmt.Errorf("failed to close factories: %w", err)
	}

	// Service container executed.
	return nil
}
