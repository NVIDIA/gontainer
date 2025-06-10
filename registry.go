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
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

// registry contains all defined factories metadata.
type registry struct {
	factories []*factory
	sequence  []*factory
	events    Events
	mutex     sync.Mutex
}

// registerFactory registers factory in the registry.
func (r *registry) registerFactory(factory *factory) {
	r.factories = append(r.factories, factory)
}

// validateFactories validates registered factories.
// Validate checks for availability of all non-optional types
// and for possible circular dependencies between factories.
func (r *registry) validateFactories() error {
	// Prepare result errors slice.
	var errs []error

	// Validate all input types are resolvable.
	for _, factory := range r.factories {
		for index, inType := range factory.inTypes {
			// Is this type a special factory context type?
			if isContextInterface(inType) {
				continue
			}

			// Is this type wrapped to the `Optional[type]`?
			_, isOptional := isOptionalType(inType)
			if isOptional {
				continue
			}

			// Is this type wrapped to the `Multiple[type]`?
			_, isMultiple := isMultipleType(inType)
			if isMultiple {
				continue
			}

			// Is a factory for this type could be resolved?
			typeFactories, _ := r.findFactoriesFor(inType)
			if len(typeFactories) == 0 {
				errs = append(errs, fmt.Errorf(
					"failed to validate argument '%s' (index %d) of factory '%s' from '%s': %w",
					inType, index, factory.source.name, factory.source.source, ErrServiceNotResolved,
				))
				continue
			}
		}
	}

	// Validate all output types are unique.
	for _, factory := range r.factories {
		for index, outType := range factory.outTypes {
			// Factories returning `any` could be duplicated.
			if isEmptyInterface(outType) {
				continue
			}

			// Validate uniqueness of the every factory output type.
			typeFactories, _ := r.findFactoriesFor(outType)
			if len(typeFactories) > 1 {
				errs = append(errs, fmt.Errorf(
					"failed to validate output '%s' (index %d) of factory '%s' from '%s': %w",
					outType, index, factory.source.name, factory.source.source, ErrServiceDuplicated,
				))
			}
		}
	}

	// Validate for circular dependencies.
	for index := range r.factories {
		factories := []*factory{r.factories[index]}
	recursion:
		for len(factories) > 0 {
			factory := factories[0]
			factories = factories[1:]

			for _, inType := range factory.inTypes {
				// Is this type a special factory context type?
				if isContextInterface(inType) {
					continue
				}

				// Is this type wrapped to the `Optional[type]`?
				innerType, isOptional := isOptionalType(inType)
				if isOptional {
					inType = innerType
				}

				// Is this type wrapped to the `Multiple[type]`?
				innerType, isMultiple := isMultipleType(inType)
				if isMultiple {
					inType = innerType
				}

				// Walk through all factories for this in argument type.
				typeFactories, _ := r.findFactoriesFor(inType)
				for _, factoryForType := range typeFactories {
					if factoryForType == r.factories[index] {
						errs = append(errs, fmt.Errorf(
							"failed to validate factory '%s' from '%s': %w",
							r.factories[index].source.name, r.factories[index].source.source,
							ErrCircularDependency,
						))
						break recursion
					}
				}

				factories = append(factories, typeFactories...)
			}
		}
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// spawnFactories spawns all factories in the registry.
func (r *registry) spawnFactories() error {
	// Spawn all factories with registration order.
	// If the factory requires a dependency not yet spawned,
	// but found in the registry, then it will be spawned
	// before the current factory and this fact will be
	// recorded in the `sequence` slice.
	for _, factory := range r.factories {
		if err := r.spawnFactory(factory); err != nil {
			return fmt.Errorf(
				"failed to spawn services of '%s' from '%s': %w",
				factory.source.name, factory.source.source, err,
			)
		}
	}

	// Only the first error is returned.
	return nil
}

// closeFactories closes all factories in the reverse order.
func (r *registry) closeFactories() error {
	// Prepare result errors slice.
	var errs []error

	// Close all spawned factories in the reverse order.
	for index := len(r.sequence) - 1; index >= 0; index-- {
		factory := r.sequence[index]

		// Cancel factory context before calling close on services.
		// This allows background factory functions to unblock from
		// the waiting of context done channel and finish work.
		factory.cancel()

		// Handle every spawned factory output value.
		for outIndex, outValue := range factory.getOutValues() {
			// Get the factory result object interface.
			service := outValue.Interface()

			// Close service implementing `Close() error` interface.
			// Service functions will wait for the function return.
			if closer, ok := service.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					errs = append(errs, fmt.Errorf(
						"failed to close service '%s' (index %d) of factory '%s' from '%s': %w",
						outValue.Type(), outIndex, factory.source.name, factory.source.source, err),
					)
				}
			}

			// Close service implementing `Close()` interface.
			if closer, ok := service.(interface{ Close() }); ok {
				closer.Close()
			}
		}
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// resolveService resolves and returns the service based on the type.
func (r *registry) resolveService(serviceType reflect.Type) (reflect.Value, error) {
	// Is a target type - optional container?
	innerType, isOptional := isOptionalType(serviceType)
	if isOptional {
		return r.resolveOptional(serviceType, innerType)
	}

	// Is a target type - multiple container?
	innerType, isMultiple := isMultipleType(serviceType)
	if isMultiple {
		return r.resolveMultiple(serviceType, innerType)
	}

	// Resolve regular service.
	return r.resolveRegular(serviceType)
}

// resolveOptional resolves a service wrapped with an optional type.
func (r *registry) resolveOptional(optionalType, serviceType reflect.Type) (reflect.Value, error) {
	serviceValues, err := r.resolveByType(serviceType)
	if err != nil {
		return reflect.Value{}, err
	}

	// Resolve of regular types leads to an error if the factory for the type is not registered
	// in the container. Resolve of optional types works differently: if the optional type is
	// not found, then a zero-value box should be returned instead. For example, resolving an
	// unregistered type `Config` triggers an error, while resolving `gontainer.Optional[Config]`
	// returns a zero-value box.
	if len(serviceValues) == 0 {
		zeroValue := reflect.New(serviceType).Elem()
		return newOptionalValue(optionalType, zeroValue), nil
	}

	// Return resolved service in an optional box type.
	// For example, if a factory accepts `gontainer.Optional[Config]`,
	// then it is required to wrap the `Config` with an `Optional` struct.
	return newOptionalValue(optionalType, serviceValues[0]), nil
}

// resolveMultiple resolves all services fits to the multiple type.
func (r *registry) resolveMultiple(multipleType, serviceType reflect.Type) (reflect.Value, error) {
	serviceValues, err := r.resolveByType(serviceType)
	if err != nil {
		return reflect.Value{}, err
	}
	return newMultipleValue(multipleType, serviceValues), nil
}

// resolveRegular resolves a regular service.
func (r *registry) resolveRegular(serviceType reflect.Type) (reflect.Value, error) {
	// Resolve all services by specified type.
	serviceValues, err := r.resolveByType(serviceType)
	if err != nil {
		return reflect.Value{}, err
	}

	// Resolve of regular types leads to an error if the factory for the type is not registered
	// in the container. Resolve of optional types works differently: if the optional type is
	// not found, then a zero-value box should be returned instead. For example, resolving an
	// unregistered type `Config` triggers an error, while resolving `gontainer.Optional[Config]`
	// returns a zero-value box.
	if len(serviceValues) == 0 {
		return reflect.Value{}, fmt.Errorf("%w: '%s'", ErrServiceNotResolved, serviceType)
	}

	// Pick first found service value.
	return serviceValues[0], nil
}

// resolveByType resolves all service fits to specified type.
func (r *registry) resolveByType(serviceType reflect.Type) ([]reflect.Value, error) {
	// Lookup factory definition by an output service type.
	factories, outIndexes := r.findFactoriesFor(serviceType)
	services := make([]reflect.Value, 0, len(factories))

	for index, factory := range factories {
		// Handle found factory definition.
		if err := r.spawnFactory(factory); err != nil {
			return nil, fmt.Errorf("failed to spawn factory '%s': %w", factory.funcType, err)
		}

		// Handle services from factory outputs.
		outValues := factory.getOutValues()
		for _, outIndex := range outIndexes[index] {
			services = append(services, outValues[outIndex])
		}
	}

	return services, nil
}

// findFactoriesFor lookups for all factories for an output type in the registry.
func (r *registry) findFactoriesFor(serviceType reflect.Type) ([]*factory, [][]int) {
	var records []*factory
	var indices [][]int

	// Lookup for factories in the registry.
	for _, record := range r.factories {
		var indexes []int
		for index, resultType := range record.outTypes {
			// If requested type matched.
			if resultType == serviceType {
				records = append(records, record)
				indexes = append(indexes, index)
				continue
			}

			// if requested type implements a non-empty interface.
			if isNonEmptyInterface(serviceType) {
				if resultType.Implements(serviceType) {
					records = append(records, record)
					indexes = append(indexes, index)
					continue
				}
			}
		}
		if len(indexes) > 0 {
			indices = append(indices, indexes)
		}
	}

	// Return matched factories and indices.
	return records, indices
}

// spawnFactory instantiates specified factory definition.
func (r *registry) spawnFactory(factory *factory) error {
	// Protect from cyclic dependencies.
	if getStackDepth() >= stackDepthLimit {
		return ErrStackLimitReached
	}

	// Check factory already spawned.
	if factory.getSpawned() {
		return nil
	}

	// Get or spawn factory input values recursively.
	inValues := make([]reflect.Value, 0, len(factory.inTypes))
	for _, inType := range factory.inTypes {
		// Handle specified `context.Context` as a special case.
		if isContextInterface(inType) {
			ctxValue := reflect.ValueOf(factory.ctx)
			inValues = append(inValues, ctxValue)
			continue
		}

		// Resolve factory input dependency.
		inValue, err := r.resolveService(inType)
		if err != nil {
			return fmt.Errorf("failed to resolve service: %w", err)
		}

		inValues = append(inValues, inValue)
	}

	// Spawn the factory using the factory input arguments.
	outValues := factory.funcValue.Call(inValues)
	errorValue := reflect.Value{}

	// Read optional factory error value.
	if factory.outError {
		errorValue = outValues[len(outValues)-1]
		outValues = outValues[0 : len(outValues)-1]
	}

	// Handle factory output error if present.
	if factory.outError && !errorValue.IsNil() {
		err, _ := errorValue.Interface().(error)
		return fmt.Errorf("%w: %w", ErrFactoryReturnedError, err)
	}

	// Handle factory out functions as regular objects.
	for outIndex, outValue := range outValues {
		if serviceFuncValue, ok := isServiceFunc(outValue); ok {
			serviceFuncResult, err := startServiceFunc(serviceFuncValue)
			if err != nil {
				return fmt.Errorf("failed to start factory func: %w", err)
			}
			outValues[outIndex] = serviceFuncResult
		}
	}

	// Save the factory spawn status.
	factory.setSpawned(true)

	// Save the factory out values.
	factory.setOutValues(outValues)

	// Save the factory spawn order.
	r.mutex.Lock()
	r.sequence = append(r.sequence, factory)
	r.mutex.Unlock()

	return nil
}

// function represents service func return value wrapper.
type funcResult chan error

// Close awaits service function and returns result error.
func (f funcResult) Close() error {
	return <-f
}

// isNonEmptyInterface returns true when argument is an interface with methods.
func isNonEmptyInterface(typ reflect.Type) bool {
	return typ.Kind() == reflect.Interface && typ.NumMethod() > 0
}

// isEmptyInterface returns true when argument is an `any` interface.
func isEmptyInterface(typ reflect.Type) bool {
	return typ.Kind() == reflect.Interface && typ.NumMethod() == 0
}

// isErrorInterface returns true when argument is an `error` interface.
func isErrorInterface(typ reflect.Type) bool {
	ctxType := reflect.TypeOf((*error)(nil)).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(ctxType)
}

// isContextInterface returns true when argument is a context interface.
func isContextInterface(typ reflect.Type) bool {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(ctxType)
}

// isServiceFunc returns true when the argument is a service function.
// The `outValue` can be an `any` type with a function in the value
// (when the factory declares `any` return type), or the `function` type
// (when the factory declares explicit return type).
func isServiceFunc(outValue reflect.Value) (reflect.Value, bool) {
	// Unbox the value if it is an any interface.
	if isEmptyInterface(outValue.Type()) {
		outValue = outValue.Elem()
	}

	// Check if the result value kind is a func kind.
	if factoryOutValue.Kind() != reflect.Func {
		return reflect.Value{}, false
	}

	// The service func type must have no user-defined methods,
	// Otherwise it could be a service instance based on the func type
	// but with a several public methods implementing public interface.
	if outValue.NumMethod() > 0 {
		return reflect.Value{}, false
	}

	// The service func type could be just a `func()` type.
	if outValue.Type().NumOut() == 0 {
		return factoryOutValue, true
	}

	// The service func type could be a `func() error` type.
	if factoryOutValue.Type().NumOut() == 1 {
		if isErrorInterface(factoryOutValue.Type().Out(0)) {
			return outValue, true
		}
	}

	// All other types like `func(name string) (MyType, error)`
	// or `func() MyType` or are not service functions.
	return reflect.Value{}, false
}

// startServiceFunc wraps service function to the regular service object.
func startServiceFunc(serviceFuncValue reflect.Value) (reflect.Value, error) {
	// Prepare closable result chan as a service function replacement.
	// The service function result error will be returned from the
	// result wrapper chan `Close() error` method right to the
	// service container on the termination stage.
	funcResultChan := funcResult(make(chan error))

	// Start the service function in background.
	serviceFunc := serviceFuncValue.Interface()
	switch serviceFunc := serviceFunc.(type) {
	case func() error:
		go func() {
			err := serviceFunc()
			funcResultChan <- err
		}()
	case func():
		go func() {
			serviceFunc()
			funcResultChan <- nil
		}()
	default:
		// This case must never be reached.
		// Protected by the `isServiceFunc()`.
		return reflect.Value{}, fmt.Errorf(
			"unexpected service function signature: %T",
			serviceFuncValue.Interface(),
		)
	}

	// Return reflected value of the wrapper.
	return reflect.ValueOf(funcResultChan), nil
}

// errorType contains reflection type for error variable.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// getStackDepth returns current stack depth.
func getStackDepth() int {
	pc := make([]uintptr, stackDepthLimit+1)
	return runtime.Callers(0, pc) - 1
}

// stackDepthLimit to protect from infinite recursion.
const stackDepthLimit = 100

// ErrFactoryReturnedError declares factory returned error.
var ErrFactoryReturnedError = errors.New("factory returned error")

// ErrServiceDuplicated declares service duplicated error.
var ErrServiceDuplicated = errors.New("service duplicated")

// ErrServiceNotResolved declares service not resolved error.
var ErrServiceNotResolved = errors.New("service not resolved")

// ErrCircularDependency declares a cyclic dependency error.
var ErrCircularDependency = errors.New("circular dependency")

// ErrStackLimitReached declares a reach of stack limit error.
var ErrStackLimitReached = errors.New("stack limit reached")
