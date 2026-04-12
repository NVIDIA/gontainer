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
func Run(ctx context.Context, options ...Option) error {
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
	invoker := &Invoker{registry: registry}

	// Register service resolver instance in the registry.
	if err := NewService(resolver).apply(ctx, registry); err != nil {
		return fmt.Errorf("failed to register resolver: %w", err)
	}

	// Register function invoker instance in the registry.
	if err := NewService(invoker).apply(ctx, registry); err != nil {
		return fmt.Errorf("failed to register invoker: %w", err)
	}

	// Register provided factories in the registry.
	for _, option := range options {
		if err := option.apply(ctx, registry); err != nil {
			return fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Validate all factories in the container.
	if err := registry.validateRegistry(); err != nil {
		return fmt.Errorf("failed to validate container: %w", err)
	}

	// Start all factories in the container.
	if err := registry.invokeEntrypoints(); err != nil {
		return fmt.Errorf("failed to invoke functions: %w", err)
	}

	// Close all factories in the container.
	if err := registry.closeFactories(); err != nil {
		return fmt.Errorf("failed to close factories: %w", err)
	}

	// Service container executed.
	return nil
}

// Option is the interface for container options.
type Option interface {
	apply(ctx context.Context, registry *registry) error
}

// NewFactory creates a new service load using the provided load function.
//
// The load function must be a function. It may accept dependencies as input parameters and return
// exactly one service instances, optionally followed by an error as the second return value.
//
// Example:
//
//	gontainer.NewFactory(func(db *Database) *Handler { ... })
//	gontainer.NewFactory(func(db *Database) (*Handler, error) { ... })
//	gontainer.NewFactory(func(db *Database) (*Handler, func() error) { ... })
//	gontainer.NewFactory(func(db *Database) (*Handler, func() error, error) { ... })
func NewFactory(function any) *Factory {
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)

	// Prepare factory description.
	name := fmt.Sprintf("Factory[%s]", funcValue.Type())
	source := getFuncSource(funcValue)

	// Prepare option callback.
	return &Factory{
		name:   name,
		source: source,
		fn: func(ctx context.Context, registry *registry) error {
			// Validate function type.
			if funcType.Kind() != reflect.Func {
				return fmt.Errorf("invalid type: %s", funcType)
			}

			// Prepare default value and error getters.
			var getOutType getOutTypeFn
			var getOutValue getOutValueFn
			var getOutClose getOutCloseFn
			var getOutError getOutErrorFn

			// Prepare value and error getters.
			switch {
			// Factory returns exactly one service.
			case funcType.NumOut() == 1 && isUsefulService(funcType.Out(0)):
				getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

			// Factory returns a service and an error.
			case funcType.NumOut() == 2 && isUsefulService(funcType.Out(0)) && isErrorInterface(funcType.Out(1)):
				getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[1] }

			// Factory returns a service and a close callback.
			case funcType.NumOut() == 2 && isUsefulService(funcType.Out(0)) && isCloseCallback(funcType.Out(1)):
				getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return outValues[1] }
				getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

			// Factory returns a service, a close callback and an error.
			case funcType.NumOut() == 3 && isUsefulService(funcType.Out(0)) && isCloseCallback(funcType.Out(1)) && isErrorInterface(funcType.Out(2)):
				getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return outValues[1] }
				getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[2] }

			// Factory signature is invalid.
			default:
				return fmt.Errorf("invalid signature: %s", funcType)
			}

			// Load the factory internal representation.
			state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutClose, getOutError)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", name, err)
			}

			// Register factory in the registry.
			registry.registerFactory(state)

			// Factory registered.
			return nil
		},
	}
}

// NewService creates a new service load that always returns the given singleton value.
//
// This is a convenience helper for registering preconstructed service instances
// as factories. The returned load produces the same instance on every invocation.
//
// This is useful for registering constants, mocks, or externally constructed values.
//
// Example:
//
//	logger := NewLogger()
//	gontainer.NewService(logger)
func NewService[T any](service T) *Factory {
	function := func() T { return service }
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)
	serviceType := reflect.TypeOf(&service).Elem()

	// Prepare factory description.
	name := fmt.Sprintf("Service[%s]", serviceType)
	source := serviceType.PkgPath()

	// Prepare option callback.
	return &Factory{
		name:   name,
		source: source,
		fn: func(ctx context.Context, registry *registry) error {
			// Prepare value and error getters.
			getOutType := func(outTypes []reflect.Type) reflect.Type { return funcType.Out(0) }
			getOutValue := func(outValues []reflect.Value) reflect.Value { return outValues[0] }
			getOutClose := func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
			getOutError := func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

			// Load the factory internal representation.
			state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutClose, getOutError)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", name, err)
			}

			// Register factory in the registry.
			registry.registerFactory(state)

			// Factory registered.
			return nil
		},
	}
}

// Factory is a container option that registers a service factory or singleton.
type Factory struct {
	name   string
	source string
	fn     func(ctx context.Context, registry *registry) error
}

// Name returns the human-readable name of the factory.
func (f *Factory) Name() string {
	return f.name
}

// Source returns the source package path of the factory.
func (f *Factory) Source() string {
	return f.source
}

// apply applies the factory option to the given registry.
func (f *Factory) apply(ctx context.Context, registry *registry) error {
	return f.fn(ctx, registry)
}

// NewEntrypoint creates a new factory which will be called by the container.
//
// Example:
//
//	gontainer.NewEntrypoint(func(db *Database) error { ... })
//	gontainer.NewEntrypoint(func(db *Database) { ... })
func NewEntrypoint(function any) *Entrypoint {
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)

	// Prepare factory description.
	name := fmt.Sprintf("Entrypoint[%s]", funcValue.Type())
	source := getFuncSource(funcValue)

	// Prepare option callback.
	return &Entrypoint{
		name:   name,
		source: source,
		fn: func(ctx context.Context, registry *registry) error {
			// Validate function type.
			if funcType.Kind() != reflect.Func {
				return fmt.Errorf("invalid type: %s", funcType)
			}

			// Prepare default value and error getters.
			var getOutType getOutTypeFn
			var getOutValue getOutValueFn
			var getOutClose getOutCloseFn
			var getOutError getOutErrorFn

			// Prepare value and error getters.
			switch {
			// Function returns nothing.
			case funcType.NumOut() == 0:
				getOutType = func(outTypes []reflect.Type) reflect.Type { return nil }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

			// Function returns an error.
			case funcType.NumOut() == 1 && isErrorInterface(funcType.Out(0)):
				getOutType = func(outTypes []reflect.Type) reflect.Type { return nil }
				getOutValue = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
				getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[0] }

			// Function signature is invalid.
			default:
				return fmt.Errorf("invalid signature: %s", funcType)
			}

			// Load the factory internal representation.
			state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutClose, getOutError)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", name, err)
			}

			// Register entrypoint in the registry.
			registry.registerEntrypoint(state)

			// Entrypoint registered.
			return nil
		},
	}
}

// Entrypoint is a container option that registers an entrypoint function.
type Entrypoint struct {
	name   string
	source string
	fn     func(ctx context.Context, registry *registry) error
}

// Name returns the human-readable name of the entrypoint.
func (e *Entrypoint) Name() string {
	return e.name
}

// Source returns the source package path of the entrypoint.
func (e *Entrypoint) Source() string {
	return e.source
}

// apply applies the entrypoint option to the given registry.
func (e *Entrypoint) apply(ctx context.Context, registry *registry) error {
	return e.fn(ctx, registry)
}
