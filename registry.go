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
	"sync"
)

// registry contains all defined factories.
type registry struct {
	factories   []*factory
	sequence    []*factory
	entrypoints []*factory
	mutex       sync.Mutex
}

// registerFactory registers factory function in the registry.
func (r *registry) registerFactory(factory *factory) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.factories = append(r.factories, factory)
}

// registerEntrypoint registers entrypoint function in the registry.
func (r *registry) registerEntrypoint(factory *factory) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.entrypoints = append(r.entrypoints, factory)
}

// validateRegistry validates all registered state.
// Validate checks for availability of all non-optional types
// and for possible circular dependencies between factories.
func (r *registry) validateRegistry() error {
	// Prepare result errors slice.
	var errs []error

	// Combine all factories and functions for validation.
	allFactories := make([]*factory, 0, len(r.factories)+len(r.entrypoints))
	allFactories = append(allFactories, r.factories...)
	allFactories = append(allFactories, r.entrypoints...)

	// Validate all input types are resolvable.
	for _, factory := range allFactories {
		for _, inType := range factory.inTypes {
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

			// Could a factory for this type be resolved?
			foundFactories := r.findFactories(inType)
			if len(foundFactories) == 0 {
				errs = append(errs, fmt.Errorf(
					"%s from '%s': argument '%s': %w",
					factory.name, factory.source, inType,
					ErrDependencyNotResolved,
				))
				continue
			}
		}
	}

	// Validate all output types are unique.
	for _, factory := range r.factories {
		// Prepare factory output type.
		outType := factory.getOutType()

		// Skip factories without output type.
		if outType == nil {
			continue
		}

		// Validate uniqueness of the factory output type.
		factories := r.findFactories(outType)
		if len(factories) > 1 {
			errs = append(errs, fmt.Errorf(
				"%s from '%s': output '%s': %w",
				factory.name, factory.source, outType,
				ErrFactoryTypeDuplicated,
			))
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
				typeFactories := r.findFactories(inType)
				for _, factoryForType := range typeFactories {
					if factoryForType == r.factories[index] {
						errs = append(errs, fmt.Errorf(
							"%s from '%s': %w",
							r.factories[index].name,
							r.factories[index].source,
							ErrCircularDependency,
						))
						break recursion
					}
				}

				factories = append(factories, typeFactories...)
			}
		}
	}

	// Validate for entrypoints count.
	if len(r.entrypoints) == 0 {
		errs = append(errs, ErrNoEntrypointsProvided)
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// invokeEntrypoints invokes registered entrypoints.
func (r *registry) invokeEntrypoints() error {
	// Prepare result errors slice.
	var errs []error

	// Invoke all functions in the registry.
	for _, factory := range r.entrypoints {
		// Invoke the factory.
		if err := r.invokeFactory(factory); err != nil {
			errs = append(errs, fmt.Errorf(
				"%s from '%s': invoke: %w",
				factory.name, factory.source, err,
			))

			// The entrypoint was not invoked.
			continue
		}

		// Handle factory error.
		if err := factory.getOutError(); err != nil {
			errs = append(errs, fmt.Errorf(
				"%s from '%s': %w: %w",
				factory.name, factory.source,
				ErrEntrypointReturnedError, err,
			))
		}
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// closeFactories closes all factories in the reverse order.
func (r *registry) closeFactories() error {
	// Prepare result errors slice.
	var errs []error

	// Close all spawned factories in the reverse order.
	for index := len(r.sequence) - 1; index >= 0; index-- {
		factory := r.sequence[index]

		// Invoke close callback function.
		if err := factory.getOutClose()(); err != nil {
			errs = append(errs, fmt.Errorf(
				"%s from '%s': close: %w",
				factory.name, factory.source, err,
			))
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
		return newOptionalZero(optionalType), nil
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
	resolvedValues, err := r.resolveByType(serviceType)
	if err != nil {
		return reflect.Value{}, err
	}

	// Resolve of regular types leads to an error if the factory for the type is not registered
	// in the container. Resolve of optional types works differently: if the optional type is
	// not found, then a zero-value box should be returned instead. For example, resolving an
	// unregistered type `Config` triggers an error, while resolving `gontainer.Optional[Config]`
	// returns a zero-value box.
	if len(resolvedValues) == 0 {
		return reflect.Value{}, fmt.Errorf("%w: '%s'", ErrDependencyNotResolved, serviceType)
	}

	// Pick first found service value.
	return resolvedValues[0], nil
}

// resolveByType resolves all service fits to specified type.
func (r *registry) resolveByType(serviceType reflect.Type) ([]reflect.Value, error) {
	// Lookup factory definition by an output type.
	factories := r.findFactories(serviceType)
	resolvedValues := make([]reflect.Value, 0, len(factories))

	// Spawn all found factories.
	for _, factory := range factories {
		// Handle found factory definition.
		if err := r.spawnFactory(factory); err != nil {
			return nil, fmt.Errorf(
				"%s from '%s': spawn: %w",
				factory.name, factory.source, err,
			)
		}

		// Handle error returned by the factory.
		if err := factory.getOutError(); err != nil {
			return nil, fmt.Errorf(
				"%s from '%s': %w: %w",
				factory.name, factory.source,
				ErrFactoryReturnedError, err,
			)
		}

		// Handle the factory result output.
		resolvedValues = append(resolvedValues, factory.getOutValue())
	}

	return resolvedValues, nil
}

// findFactories lookups for all factories for an output type in the registry.
func (r *registry) findFactories(serviceType reflect.Type) []*factory {
	var factories []*factory

	// Lookup for factories in the registry.
	for _, factory := range r.factories {
		outType := factory.getOutType()

		// Skip factories without an output type.
		if outType == nil {
			continue
		}

		// Desired service type matched.
		if outType == serviceType {
			factories = append(factories, factory)
			continue
		}

		// Desired service type implements an interface.
		if serviceType.Kind() == reflect.Interface {
			if outType.Implements(serviceType) {
				factories = append(factories, factory)
				continue
			}
		}
	}

	// Return matched factories.
	return factories
}

// spawnFactory instantiates specified factory definition.
func (r *registry) spawnFactory(factory *factory) error {
	// Lock the factory spawn mutex.
	factory.spawnMu.Lock()
	defer factory.spawnMu.Unlock()

	// Check factory already spawned.
	if factory.getIsSpawned() {
		return nil
	}

	// Invoke the factory.
	err := r.invokeFactory(factory)
	if err != nil {
		return err
	}

	// Save the factory spawn status.
	factory.setIsSpawned(true)

	// Save the factory spawn order.
	r.mutex.Lock()
	r.sequence = append(r.sequence, factory)
	r.mutex.Unlock()

	return nil
}

// invokeFactory calls the factory function and returns output values.
func (r *registry) invokeFactory(factory *factory) error {
	// Get or spawn factory input values recursively.
	inValues := make([]reflect.Value, 0, len(factory.inTypes))
	for _, inType := range factory.inTypes {
		// Resolve factory input dependency.
		inValue, err := r.resolveService(inType)
		if err != nil {
			return err
		}

		// Append resolved input value.
		inValues = append(inValues, inValue)
	}

	// Call the factory using input arguments.
	outValues := factory.funcValue.Call(inValues)

	// Set factory output values.
	factory.setOutValues(outValues)

	// Factory invoked successfully.
	return nil
}

// isEmptyInterface returns true when argument is an `any` interface.
func isEmptyInterface(typ reflect.Type) bool {
	return typ.Kind() == reflect.Interface && typ.NumMethod() == 0
}

// isErrorInterface returns true when argument is an error interface.
func isErrorInterface(typ reflect.Type) bool {
	errType := reflect.TypeOf((*error)(nil)).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(errType)
}

// isCloseCallback returns true when argument is a close callback function.
func isCloseCallback(typ reflect.Type) bool {
	refType := reflect.TypeOf(func() error { return nil })
	return typ.Kind() == reflect.Func && typ == refType
}
