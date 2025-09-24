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

// Run executes service container instance with a set of configured services.
// The `factories` specifies factories for services with dependency resolution.
func Run(ctx context.Context, options ...Option) error {
	// Prepare container context ignoring the cancelling.
	// When cancelled, it closes `container.Done()` channel
	// and unblocks any waiting read from `container.Done()`.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	// Prepare services registry.
	registry := &registry{}

	// Register service resolver.
	resolver := &Resolver{registry: registry}
	if err := NewService(resolver)(ctx, registry); err != nil {
		return fmt.Errorf("failed to register service resolver: %w", err)
	}

	// Register function invoker.
	invoker := &Invoker{resolver: resolver}
	if err := NewService(invoker)(ctx, registry); err != nil {
		return fmt.Errorf("failed to register function invoker: %w", err)
	}

	// Register provided factories.
	for _, option := range options {
		if err := option(ctx, registry); err != nil {
			return fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Validate all factories in the registry.
	if err := registry.validateRegistry(); err != nil {
		return fmt.Errorf("failed to validate container: %w", err)
	}

	// Start all factories in the container.
	if err := registry.invokeFunctions(); err != nil {
		return fmt.Errorf("failed to invoke functions: %w", err)
	}

	return nil
}

// Option defines an option for the service container.
type Option func(ctx context.Context, registry *registry) error

// NewFactory creates a new service load using the provided load function.
//
// The load function must be a valid function. It may accept dependencies as input parameters,
// and return one or more service instances, optionally followed by an error as the last return value.
//
// Optional configuration can be applied via load options (`FactoryOpt`), such as providing additional metadata.
//
// The resulting Factory can be registered in the container.
//
// Example:
//
//	gontainer.NewFactory(func(db *Database) (*Handler, error) { ... })
//	gontainer.NewFactory(func(db *Database) *Handler { ... })
func NewFactory(function any) Option {
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)

	// Prepare factory description.
	name := fmt.Sprintf("Factory[%s]", funcValue.Type())
	source := getFuncSource(funcValue)

	// Prepare option callback.
	return func(ctx context.Context, registry *registry) error {
		// Validate function type.
		if funcType.Kind() != reflect.Func {
			return fmt.Errorf("invalid type: %s", funcType)
		}

		// Prepare default value and error getters.
		var getOutType getOutTypeFn
		var getOutValue getOutValueFn
		var getOutError getOutErrorFn

		// Prepare value and error getters.
		switch {
		// Factory returns exactly one service.
		case funcType.NumOut() == 1 && !isEmptyInterface(funcType.Out(0)):
			getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
			getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
			getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

		// Factory returns a service and an error.
		case funcType.NumOut() == 2 && !isEmptyInterface(funcType.Out(0)) && isErrorInterface(funcType.Out(1)):
			getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
			getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
			getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[1] }

		// Factory signature is invalid.
		default:
			return fmt.Errorf("invalid signature: %s", funcType)
		}

		// Load the factory internal representation.
		state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutError)
		if err != nil {
			return fmt.Errorf("failed to load factory '%s': %w", name, err)
		}

		// Register factory in the registry.
		registry.registerFactory(state)

		// Factory registered.
		return nil
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
func NewService[T any](service T) Option {
	function := func() T { return service }
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)
	serviceType := reflect.TypeOf(&service).Elem()

	// Prepare factory description.
	name := fmt.Sprintf("Service[%s]", serviceType)
	source := serviceType.PkgPath()

	// Prepare option callback.
	return func(ctx context.Context, registry *registry) error {
		// Prepare value and error getters.
		getOutType := func(outTypes []reflect.Type) reflect.Type { return funcType.Out(0) }
		getOutValue := func(outValues []reflect.Value) reflect.Value { return outValues[0] }
		getOutError := func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

		// Load the factory internal representation.
		state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutError)
		if err != nil {
			return fmt.Errorf("failed to load factory '%s': %w", name, err)
		}

		// Register factory in the registry.
		registry.registerFactory(state)

		// Factory registered.
		return nil
	}
}

// NewFunction creates a new factory which will be called by the container.
//
// Example:
//
//	gontainer.NewFunction(func(db *Database) error { ... })
//	gontainer.NewFunction(func(db *Database) { ... })
func NewFunction(function any) Option {
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)

	// Prepare factory description.
	name := fmt.Sprintf("Function[%s]", funcValue.Type())
	source := getFuncSource(funcValue)

	// Prepare option callback.
	return func(ctx context.Context, registry *registry) error {
		// Validate function type.
		if funcType.Kind() != reflect.Func {
			return fmt.Errorf("invalid type: %s", funcType)
		}

		// Prepare default value and error getters.
		var getOutType getOutTypeFn
		var getOutValue getOutValueFn
		var getOutError getOutErrorFn

		// Prepare value and error getters.
		switch {
		// Function returns nothing.
		case funcType.NumOut() == 0:
			getOutType = func(outTypes []reflect.Type) reflect.Type { return nil }
			getOutValue = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
			getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

		// Function returns an error.
		case funcType.NumOut() == 1 && isErrorInterface(funcType.Out(0)):
			getOutType = func(outTypes []reflect.Type) reflect.Type { return nil }
			getOutValue = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
			getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[0] }

		// Function signature is invalid.
		default:
			return fmt.Errorf("invalid signature: %s", funcType)
		}

		// Load the factory internal representation.
		state, err := newFactory(ctx, name, source, funcValue, getOutType, getOutValue, getOutError)
		if err != nil {
			return fmt.Errorf("failed to load factory '%s': %w", name, err)
		}

		// Register factory in the registry.
		registry.registerFunction(state)

		// Factory registered.
		return nil
	}
}
