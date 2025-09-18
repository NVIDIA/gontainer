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
	"sync"
)

// New returns new container instance with a set of configured services.
// The `factories` specifies factories for services with dependency resolution.
func New(ctx context.Context, factories ...*Factory) (result *Container, err error) {
	// Prepare container context ignoring the cancelling.
	// When cancelled, it closes `container.Done()` channel
	// and unblocks any waiting read from `container.Done()`.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	// Cancel context only when returning an error.
	// Otherwise, in will be cancelled by container.
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// Prepare services registry instance.
	registry := &registry{}

	// Prepare service resolver instance.
	resolver := &Resolver{registry: registry}

	// Prepare function invoker instance.
	invoker := &Invoker{resolver: resolver}

	// Prepare service container instance.
	container := &Container{
		ctx:      ctx,
		cancel:   cancel,
		resolver: resolver,
		invoker:  invoker,
		registry: registry,
	}

	// Register service resolver instance in the registry.
	if factory, err := NewService[*Resolver](resolver).factory(); err != nil {
		return nil, fmt.Errorf("failed to register service resolver: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register function invoker instance in the registry.
	if factory, err := NewService[*Invoker](invoker).factory(); err != nil {
		return nil, fmt.Errorf("failed to register function invoker: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register provided factories in the registry.
	for _, source := range factories {
		if factory, err := source.factory(); err != nil {
			return nil, fmt.Errorf("failed to register factory: %w", err)
		} else {
			registry.registerFactory(factory)
		}
	}

	// Validate all factories in the registry.
	if err := registry.validateFactories(); err != nil {
		return nil, fmt.Errorf("failed to validate factories: %w", err)
	}

	// Return service container instance.
	return container, nil
}

// Container defines the main interface for a service container.
//
// A Container is responsible for managing the lifecycle of services,
// including their initialization, shutdown, and dependency resolution.
//
// It supports both eager initialization via Start(), and lazy resolution
// via Resolver or Invoker before the container is started. Services are
// created using registered factories, and may optionally implement a Close()
// method to participate in graceful shutdown.
type Container struct {
	ctx    context.Context
	cancel context.CancelFunc
	closer sync.Once
	mutex  sync.RWMutex

	// Service resolver.
	resolver *Resolver

	// Function invoker.
	invoker *Invoker

	// Services registry.
	registry *registry
}

// Start initializes all registered services in dependency order.
// Services are instantiated via their factories.
// Returns an error if initialization fails.
func (c *Container) Start() (resultErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = c.Close()
			panic(recovered)
		}
	}()

	// Close service container immediately on error.
	defer func() {
		if resultErr != nil {
			_ = c.Close()
		}
	}()

	// Acquire write lock.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Start all factories in the container.
	startErr := c.registry.spawnFactories()

	// Handle container start error.
	if startErr != nil {
		return fmt.Errorf("failed to start services in container: %w", startErr)
	}

	return nil
}

// Close shuts down all services in reverse order of their instantiation.
// This method blocks until all services are properly closed.
func (c *Container) Close() (err error) {

	// Acquire write lock.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Init container close once.
	c.closer.Do(func() {
		// Close container context independently of errors.
		// It will unblock all concurrent close calls.
		defer c.cancel()

		// Close all spawned services in the registry.
		closeErr := c.registry.closeFactories()
		if closeErr != nil {
			err = fmt.Errorf("failed to close factories: %w", closeErr)
			return
		}
	})

	// Await container close, e.g. from concurrent close call.
	<-c.ctx.Done()

	return nil
}

// Done returns a channel that is closed after all services have been shut down.
// Useful for coordinating external shutdown logic.
func (c *Container) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Factories returns all registered service factories.
func (c *Container) Factories() []*Factory {
	// Acquire read lock.
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Collect all factory definitions.
	factories := make([]*Factory, 0, len(c.registry.factories))
	for _, factory := range c.registry.factories {
		factories = append(factories, factory.source)
	}

	return factories
}

// Services returns all currently instantiated services.
func (c *Container) Services() []any {
	// Acquire read lock.
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	select {
	case <-c.ctx.Done():
		return nil
	default:
		services := make([]any, 0, len(c.registry.factories))
		for _, factory := range c.registry.factories {
			if factory.getSpawned() {
				for _, serviceValue := range factory.getOutValues() {
					services = append(services, serviceValue.Interface())
				}
			}
		}
		return services
	}
}

// Resolver returns a service resolver for on-demand dependency injection.
// If the container is not yet started, only requested services and their
// transitive dependencies will be instantiated.
func (c *Container) Resolver() *Resolver {
	// Acquire read lock.
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	select {
	case <-c.ctx.Done():
		return nil
	default:
		return c.resolver
	}
}

// Invoker returns a function invoker that can call user-provided functions
// with auto-injected dependencies. Behaves lazily if the container is not started.
func (c *Container) Invoker() *Invoker {
	// Acquire read lock.
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	select {
	case <-c.ctx.Done():
		return nil
	default:
		return c.invoker
	}
}
