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
	"runtime/debug"
	"sync"
)

// Events declaration.
const (
	// ContainerStarting declares container starting event.
	ContainerStarting = "ContainerStarting"

	// ContainerStarted declares container started event.
	ContainerStarted = "ContainerStarted"

	// ContainerClosing declares container closing event.
	ContainerClosing = "ContainerClosing"

	// ContainerClosed declares container closed event.
	ContainerClosed = "ContainerClosed"

	// UnhandledPanic declares unhandled panic in container.
	UnhandledPanic = "UnhandledPanic"
)

// New returns new container instance with a set of configured services.
// The `factories` specifies factories for services with dependency resolution.
func New(factories ...*Factory) (result Container, err error) {
	// Don't accept the context in args, since it mustn't be cancelled outside.
	// Cancel of the root context will trigger cancel of all children contexts, but
	// it is unwanted behavior: services should be cancelled in strict reverse order.

	// Prepare container context.
	// When cancelled, it closes `container.Done()` channel
	// and unblocks any waiting read from `container.Done()`.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context only when returning an error.
	// Otherwise, in will be cancelled by container.
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// Prepare events broker instance.
	events := &events{
		mutex:  sync.RWMutex{},
		events: make(map[string][]handler),
	}

	// Prepare services registry instance.
	registry := &registry{events: events}

	// Prepare service resolver instance.
	resolver := &resolver{registry: registry}

	// Prepare function invoker instance.
	invoker := &invoker{resolver: resolver}

	// Prepare service container instance.
	container := &container{
		ctx:      ctx,
		cancel:   cancel,
		events:   events,
		resolver: resolver,
		invoker:  invoker,
		registry: registry,
	}

	// Trigger panic events in service container.
	defer func() {
		if recovered := recover(); recovered != nil {
			event := NewEvent(UnhandledPanic, recovered, string(debug.Stack()))
			_ = container.events.Trigger(event)
			panic(recovered)
		}
	}()

	// Register events broker instance in the registry.
	if factory, err := NewService[Events](events).factory(); err != nil {
		return nil, fmt.Errorf("failed to register events manager: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register service resolver instance in the registry.
	if factory, err := NewService[Resolver](resolver).factory(); err != nil {
		return nil, fmt.Errorf("failed to register service resolver: %w", err)
	} else {
		registry.registerFactory(factory)
	}

	// Register function invoker instance in the registry.
	if factory, err := NewService[Invoker](invoker).factory(); err != nil {
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
//
// The container also includes an internal events broker for decoupled communication
// between services.
type Container interface {
	// Start initializes all registered services in dependency order.
	// Services are instantiated via their factories.
	// Returns an error if initialization fails.
	Start() error

	// Close shuts down all services in reverse order of their instantiation.
	// This method blocks until all services are properly closed.
	Close() error

	// Done returns a channel that is closed after all services have been shut down.
	// Useful for coordinating external shutdown logic.
	Done() <-chan struct{}

	// Factories returns all registered service factories.
	Factories() []*Factory

	// Services returns all currently instantiated services.
	Services() []any

	// Events returns the container-wide event broker instance.
	Events() Events

	// Resolver returns a service resolver for on-demand dependency injection.
	// If the container is not yet started, only requested services and their
	// transitive dependencies will be instantiated.
	Resolver() Resolver

	// Invoker returns a function invoker that can call user-provided functions
	// with auto-injected dependencies. Behaves lazily if the container is not started.
	Invoker() Invoker
}

// container implements service container.
type container struct {
	ctx    context.Context
	cancel context.CancelFunc
	closer sync.Once
	mutex  sync.RWMutex

	// Events broker.
	events Events

	// Service resolver.
	resolver Resolver

	// Function invoker.
	invoker Invoker

	// Services registry.
	registry *registry
}

// Start initializes every service in the container.
func (c *container) Start() (resultErr error) {
	// Trigger panic events in service container.
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = c.events.Trigger(NewEvent(UnhandledPanic, recovered, string(debug.Stack())))
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

	// Trigger container starting event.
	if err := c.events.Trigger(NewEvent(ContainerStarting)); err != nil {
		return fmt.Errorf("failed to trigger container starting event: %w", err)
	}

	// Start all factories in the container.
	startErr := c.registry.spawnFactories()

	// Trigger container started event.
	if err := c.events.Trigger(NewEvent(ContainerStarted, startErr)); err != nil {
		return fmt.Errorf("failed to trigger container started event: %w", err)
	}

	// Handle container start error.
	if startErr != nil {
		return fmt.Errorf("failed to start services in container: %w", startErr)
	}

	return nil
}

// Close closes service container with all services.
// Blocks invocation until the container is closed.
func (c *container) Close() (err error) {
	// Trigger panic events in service container.
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = c.events.Trigger(NewEvent(UnhandledPanic, recovered, string(debug.Stack())))
			panic(recovered)
		}
	}()

	// Acquire write lock.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Init container close once.
	c.closer.Do(func() {
		// Close container context independently of errors.
		// It will unblock all concurrent close calls.
		defer c.cancel()

		// Trigger container closing event.
		if triggerErr := c.events.Trigger(NewEvent(ContainerClosing)); triggerErr != nil {
			err = fmt.Errorf("failed to trigger container closing event: %w", triggerErr)
			return
		}

		// Close all spawned services in the registry.
		closeErr := c.registry.closeFactories()
		if closeErr != nil {
			err = fmt.Errorf("failed to close factories: %w", closeErr)
			return
		}

		// Trigger container closed event.
		if triggerErr := c.events.Trigger(NewEvent(ContainerClosed, closeErr)); triggerErr != nil {
			err = fmt.Errorf("failed to trigger container closed event: %w", triggerErr)
			return
		}
	})

	// Await container close, e.g. from concurrent close call.
	<-c.ctx.Done()

	return nil
}

// Done is closing after closing of all services.
func (c *container) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Factories returns all defined factories.
func (c *container) Factories() []*Factory {
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

// Services returns all spawned services.
func (c *container) Services() []any {
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

// Events returns events broker instance.
func (c *container) Events() Events {
	// Acquire read lock.
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	select {
	case <-c.ctx.Done():
		return nil
	default:
		return c.events
	}
}

// Resolver returns service resolver instance.
func (c *container) Resolver() Resolver {
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

// Invoker returns function invoker instance.
func (c *container) Invoker() Invoker {
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
