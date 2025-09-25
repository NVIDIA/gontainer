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
	"reflect"
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

// NewFactory creates a new service factory using the provided factory function.
//
// The factory function must be a valid function. It may accept dependencies as input parameters,
// and return one or more service instances, optionally followed by an error as the last return value.
//
// The resulting Factory can be registered in the container.
//
// Example:
//
//	gontainer.NewFactory(func(db *Database) (*Handler, error), gontainer.WithTag("http"))
func NewFactory(function any) *Factory {
	funcValue := reflect.ValueOf(function)
	factory := &Factory{
		fn:     function,
		name:   fmt.Sprintf("Factory[%s]", funcValue.Type()),
		source: getFuncSource(funcValue),
	}
	return factory
}

// NewService creates a new service factory that always returns the given singleton value.
//
// This is a convenience helper for registering preconstructed service instances
// as factories. The returned factory produces the same instance on every invocation.
//
// This is useful for registering constants, mocks, or externally constructed values.
//
// Example:
//
//	logger := NewLogger()
//	gontainer.NewService(logger)
func NewService[T any](service T) *Factory {
	dataType := reflect.TypeOf(&service).Elem()
	factory := &Factory{
		fn:     func() T { return service },
		name:   fmt.Sprintf("Service[%s]", dataType),
		source: dataType.PkgPath(),
	}
	return factory
}
