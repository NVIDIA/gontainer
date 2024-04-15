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

// New returns new container instance with a set of configured services.
// The `factories` specifies factories for services with dependency resolution.
func New(factories ...*Factory) (result Container, err error) {
	// Don't accept the context in args, since it mustn't be cancelled outside.
	// Cancel of the root context will trigger cancel of all children contexts, but
	// it is unwanted behavior: services should be cancelled in strict reverse order.

	// Prepare cancellable app context.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context only when returning an error.
	// Otherwise, in will be cancelled by container.
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// Prepare events broker instance.
	events := events{}

	// Prepare services registry instance.
	registry := &registry{events: events}

	// Prepare function invoker instance.
	invoker := &invoker{ctx: ctx, registry: registry}

	// Prepare service container instance.
	container := &container{
		ctx:      ctx,
		cancel:   cancel,
		events:   events,
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

	// Register service container instance in the registry.
	if err := container.registry.registerFactory(ctx, NewService[Invoker](invoker)); err != nil {
		return nil, fmt.Errorf("failed to register service invoker: %w", err)
	}

	// Register events broker instance in the registry.
	if err := container.registry.registerFactory(ctx, NewService[Events](events)); err != nil {
		return nil, fmt.Errorf("failed to register events service: %w", err)
	}

	// Register provided factories in the registry.
	for _, factory := range factories {
		if err := container.registry.registerFactory(ctx, factory); err != nil {
			return nil, fmt.Errorf("failed to register factory: %w", err)
		}
	}

	// Return service container instance.
	return container, nil
}

// Container defines service container interface.
type Container interface {
	// Start initializes every service in the container.
	Start() error

	// Close closes service container with all services.
	// Blocks invocation until the container is closed.
	Close() error

	// Done is closing after closing of all services.
	Done() <-chan struct{}

	// Events returns events broker instance.
	Events() Events
}

// Optional defines optional service dependency.
type Optional[T any] struct {
	value T
}

// Get returns optional service instance.
func (o Optional[T]) Get() T {
	return o.value
}

// container implements service container.
type container struct {
	ctx    context.Context
	cancel context.CancelFunc
	closer sync.Once

	// Events broker.
	events Events

	// Services registry.
	registry *registry
}

// Start initializes every service in the container.
func (c *container) Start() error {
	// Trigger panic events in service container.
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = c.events.Trigger(NewEvent(UnhandledPanic, recovered, string(debug.Stack())))
			panic(recovered)
		}
	}()

	// Trigger container starting event.
	if err := c.events.Trigger(NewEvent(ContainerStarting)); err != nil {
		return fmt.Errorf("failed to trigger container starting event: %w", err)
	}

	// Start all factories in the container.
	startErr := c.registry.startFactories()

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

	return
}

// Done is closing after closing of all services.
func (c *container) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Events returns events broker instance.
func (c *container) Events() Events {
	return c.events
}
