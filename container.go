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
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
)

// Run runs a container with a set of configured factories.
//
// Run registers the provided options, validates the registry, invokes
// entrypoints synchronously, and then tears down all spawned factories
// in reverse order. It returns when all entrypoints have returned and
// teardown has completed.
func Run(options ...Option) error {
	// Prepare services registry instance.
	registry := &registry{}

	// Prepare service resolver instance.
	resolver := &Resolver{registry: registry}

	// Prepare function invoker instance.
	invoker := &Invoker{registry: registry}

	// Register service resolver instance in the registry.
	if err := NewService(resolver).apply(registry); err != nil {
		return err
	}

	// Register function invoker instance in the registry.
	if err := NewService(invoker).apply(registry); err != nil {
		return err
	}

	// Register provided factories in the registry.
	for _, option := range options {
		if err := option.apply(registry); err != nil {
			return err
		}
	}

	// Validate all factories in the container.
	if err := registry.validateRegistry(); err != nil {
		return err
	}

	// Invoke all specified entrypoints.
	invokeErr := registry.invokeEntrypoints()

	// Close all factories in the reverse order.
	closeErr := registry.closeFactories()

	// Return the joined invocation and teardown errors.
	if invokeErr != nil && closeErr != nil {
		return errors.Join(invokeErr, closeErr)
	}

	// Return the invocation error if it occurred.
	if invokeErr != nil {
		return invokeErr
	}

	// Return the teardown error if it occurred.
	if closeErr != nil {
		return closeErr
	}

	// No errors occurred.
	return nil
}

// Option is the interface for container options.
type Option interface {
	apply(registry *registry) error
}

// NewFactory creates a new service factory using the provided load function.
//
// The load function may accept dependencies as input parameters and must return
// exactly one service instance, optionally followed by a cleanup callback and/or
// an error. The supported signatures are:
//
//	gontainer.NewFactory(func(/* deps */) *Service { ... })
//	gontainer.NewFactory(func(/* deps */) (*Service, error) { ... })
//	gontainer.NewFactory(func(/* deps */) (*Service, func() error) { ... })
//	gontainer.NewFactory(func(/* deps */) (*Service, func() error, error) { ... })
//
// NewFactory validates its function argument at the public API boundary and
// panics on a programmer error: when function is an untyped nil, is not a
// function, is a typed nil function, or has an unsupported signature. Failures
// that occur later for a valid factory (during dependency resolution, graph construction,
// factory execution, or cleanup) are reported as an error from Run, never as a panic.
func NewFactory(function any, opts ...FactoryOption) *Factory {
	// Examples shown to the caller on misuse.
	const examples = "Valid usage:\n" +
		"  gontainer.NewFactory(func(/* deps */) *Service { ... })\n" +
		"  gontainer.NewFactory(func(/* deps */) (*Service, error) { ... })\n" +
		"  gontainer.NewFactory(func(/* deps */) (*Service, func() error) { ... })\n" +
		"  gontainer.NewFactory(func(/* deps */) (*Service, func() error, error) { ... })"

	// Validate the function is not a nil.
	funcType := reflect.TypeOf(function)
	if funcType == nil {
		panic(fmt.Sprintf("%s NewFactory: expected a function, got nil\n\n%s", panicPrefix, examples))
	}

	// Validate the function type is a function.
	if funcType.Kind() != reflect.Func {
		panic(fmt.Sprintf("%s NewFactory: expected a function, got %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Validate the function value is not a typed nil.
	funcValue := reflect.ValueOf(function)
	if funcValue.IsNil() {
		panic(fmt.Sprintf("%s NewFactory: expected a non-nil function, got nil %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Prepare factory description.
	name := fmt.Sprintf("Factory[%s]", funcType)
	source := getCallerSource(1)

	// Prepare factory settings.
	settings := factorySettings{}
	for _, opt := range opts {
		opt.applyFactory(&settings)
	}

	// Prepare default value and error getters.
	var getOutType getOutTypeFn
	var getOutValue getOutValueFn
	var getOutClose getOutCloseFn
	var getOutError getOutErrorFn

	// Prepare value and error getters.
	switch {
	// Factory returns exactly one service.
	case funcType.NumOut() == 1 && !isEmptyInterface(funcType.Out(0)) && !isErrorInterface(funcType.Out(0)):
		getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
		getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
		getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
		getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

	// Factory returns a service and an error.
	case funcType.NumOut() == 2 && !isEmptyInterface(funcType.Out(0)) && !isErrorInterface(funcType.Out(0)) && isErrorInterface(funcType.Out(1)):
		getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
		getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
		getOutClose = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
		getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[1] }

	// Factory returns a service and a close callback.
	case funcType.NumOut() == 2 && !isEmptyInterface(funcType.Out(0)) && !isErrorInterface(funcType.Out(0)) && isCloseCallback(funcType.Out(1)):
		getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
		getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
		getOutClose = func(outValues []reflect.Value) reflect.Value { return outValues[1] }
		getOutError = func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

	// Factory returns a service, a close callback and an error.
	case funcType.NumOut() == 3 && !isEmptyInterface(funcType.Out(0)) && !isErrorInterface(funcType.Out(0)) && isCloseCallback(funcType.Out(1)) && isErrorInterface(funcType.Out(2)):
		getOutType = func(outTypes []reflect.Type) reflect.Type { return outTypes[0] }
		getOutValue = func(outValues []reflect.Value) reflect.Value { return outValues[0] }
		getOutClose = func(outValues []reflect.Value) reflect.Value { return outValues[1] }
		getOutError = func(outValues []reflect.Value) reflect.Value { return outValues[2] }

	// Factory signature is unsupported.
	default:
		panic(fmt.Sprintf("%s NewFactory: unsupported function signature %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Prepare factory instance.
	return &Factory{
		name:        name,
		source:      source,
		annotations: settings.annotations,
		register: func(registry *registry) error {
			// Load the factory internal representation.
			state, err := newFactory(
				kindFactory, name, source, funcValue,
				getOutType, getOutValue, getOutClose, getOutError,
			)
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
func NewService[T any](service T, opts ...FactoryOption) *Factory {
	function := func() T { return service }
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)
	serviceType := reflect.TypeOf(&service).Elem()

	// Prepare factory description.
	name := fmt.Sprintf("Service[%s]", serviceType)
	source := getCallerSource(1)

	// Prepare factory settings.
	settings := factorySettings{}
	for _, opt := range opts {
		opt.applyFactory(&settings)
	}

	// Prepare factory instance.
	return &Factory{
		name:        name,
		source:      source,
		annotations: settings.annotations,
		register: func(registry *registry) error {
			// Prepare value and error getters.
			getOutType := func(outTypes []reflect.Type) reflect.Type { return funcType.Out(0) }
			getOutValue := func(outValues []reflect.Value) reflect.Value { return outValues[0] }
			getOutClose := func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }
			getOutError := func(outValues []reflect.Value) reflect.Value { return reflect.Value{} }

			// Load the factory internal representation.
			state, err := newFactory(
				kindFactory, name, source, funcValue,
				getOutType, getOutValue, getOutClose, getOutError,
			)
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

// FactoryOption is an option for configuring a Factory or a Service.
type FactoryOption interface {
	// applyFactory applies the option to the factory settings.
	applyFactory(*factorySettings)
}

// Factory is a container option that registers a service factory or singleton.
type Factory struct {
	name        string
	source      string
	annotations []any
	register    func(registry *registry) error
}

// Name returns the human-readable name of the factory.
func (f *Factory) Name() string {
	return f.name
}

// Source returns the source package path of the factory.
func (f *Factory) Source() string {
	return f.source
}

// Annotations returns a copy of the associated annotations.
func (f *Factory) Annotations() []any {
	return slices.Clone(f.annotations)
}

// apply applies the factory option to the given registry.
func (f *Factory) apply(registry *registry) error {
	return f.register(registry)
}

// factorySettings holds configuration options applied to a Factory or Service.
type factorySettings struct {
	annotations []any
}

// appendAnnotation appends an annotation value.
func (s *factorySettings) appendAnnotation(value any) {
	s.annotations = append(s.annotations, value)
}

// NewEntrypoint creates a new entrypoint function which will be called by the container.
//
// The entrypoint function may accept dependencies as input parameters and must
// return nothing or a single error. The supported signatures are:
//
//	gontainer.NewEntrypoint(func(/* deps */) { ... })
//	gontainer.NewEntrypoint(func(/* deps */) error { ... })
//
// NewEntrypoint validates its function argument at the public API boundary and
// panics on a programmer error: when function is an untyped nil, is not a
// function, is a typed nil function, or has an unsupported signature. Failures
// that occur later for a valid entrypoint (during dependency resolution or entrypoint
// execution) are reported as an error from Run, never as a panic.
func NewEntrypoint(function any, opts ...EntrypointOption) *Entrypoint {
	// Examples shown to the caller on misuse.
	const examples = "Valid usage:\n" +
		"  gontainer.NewEntrypoint(func(/* deps */) { ... })\n" +
		"  gontainer.NewEntrypoint(func(/* deps */) error { ... })"

	// Validate the function is not a nil.
	funcType := reflect.TypeOf(function)
	if funcType == nil {
		panic(fmt.Sprintf("%s NewEntrypoint: expected a function, got nil\n\n%s", panicPrefix, examples))
	}

	// Validate the function type is a function.
	if funcType.Kind() != reflect.Func {
		panic(fmt.Sprintf("%s NewEntrypoint: expected a function, got %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Validate the function value is not a typed nil.
	funcValue := reflect.ValueOf(function)
	if funcValue.IsNil() {
		panic(fmt.Sprintf("%s NewEntrypoint: expected a non-nil function, got nil %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Prepare entrypoint description.
	name := fmt.Sprintf("Entrypoint[%s]", funcType)
	source := getCallerSource(1)

	// Prepare entrypoint settings.
	settings := entrypointSettings{}
	for _, opt := range opts {
		opt.applyEntrypoint(&settings)
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

	// Function signature is unsupported.
	default:
		panic(fmt.Sprintf("%s NewEntrypoint: unsupported function signature %s\n\n%s", panicPrefix, funcType, examples))
	}

	// Prepare entrypoint instance.
	return &Entrypoint{
		name:        name,
		source:      source,
		annotations: settings.annotations,
		register: func(registry *registry) error {
			// Load the factory internal representation.
			state, err := newFactory(
				kindEntrypoint, name, source, funcValue,
				getOutType, getOutValue, getOutClose, getOutError,
			)
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

// EntrypointOption is an option for configuring an Entrypoint.
type EntrypointOption interface {
	// applyEntrypoint applies the option to the entrypoint settings.
	applyEntrypoint(*entrypointSettings)
}

// Entrypoint is a container option that registers an entrypoint function.
type Entrypoint struct {
	name        string
	source      string
	annotations []any
	register    func(registry *registry) error
}

// Name returns the human-readable name of the entrypoint.
func (e *Entrypoint) Name() string {
	return e.name
}

// Source returns the source package path of the entrypoint.
func (e *Entrypoint) Source() string {
	return e.source
}

// Annotations returns a copy of the associated annotations.
func (e *Entrypoint) Annotations() []any {
	return slices.Clone(e.annotations)
}

// apply applies the entrypoint option to the given registry.
func (e *Entrypoint) apply(registry *registry) error {
	return e.register(registry)
}

// entrypointSettings holds configuration options applied to an Entrypoint.
type entrypointSettings struct {
	annotations []any
}

// appendAnnotation appends an annotation value.
func (s *entrypointSettings) appendAnnotation(value any) {
	s.annotations = append(s.annotations, value)
}

// WithAnnotation returns an option that attaches a value to a Factory or Entrypoint.
func WithAnnotation(value any) annotationOpt {
	return annotationOpt{value: value}
}

// annotationOpt is a value attachable to a Factory or Entrypoint.
type annotationOpt struct {
	value any
}

// applyFactorySettings applies the option to the factory settings.
func (m annotationOpt) applyFactory(s *factorySettings) {
	s.appendAnnotation(m.value)
}

// applyEntrypointSettings applies the option to the entrypoint settings.
func (m annotationOpt) applyEntrypoint(s *entrypointSettings) {
	s.appendAnnotation(m.value)
}

// getCallerSource returns the "<file>:<line>" of the skip-level caller.
func getCallerSource(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%d", file, line)
}
