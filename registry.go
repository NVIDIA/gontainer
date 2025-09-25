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
	"sync"
)

// registry contains all defined factories.
type registry struct {
	factories []*factory
	sequence  []*factory
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
			typeFactories := r.findFactories(inType)
			if len(typeFactories) == 0 {
				errs = append(errs, fmt.Errorf(
					"failed to validate argument '%s' (index %d) of '%s' from '%s': %w",
					inType, index, factory.name, factory.source, ErrServiceNotResolved,
				))
				continue
			}
		}
	}

	// Validate all output types are unique.
	for _, factory := range r.factories {
		// Skip factories without output type.
		if factory.getOutType() == nil {
			continue
		}

		// Validate uniqueness of the factory output type.
		factories := r.findFactories(factory.getOutType())
		if len(factories) > 1 {
			errs = append(errs, fmt.Errorf(
				"failed to validate output '%s' of '%s' from '%s': %w",
				factory.getOutType(), factory.name, factory.source, ErrServiceDuplicated,
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
				typeFactories := r.findFactories(inType)
				for _, factoryForType := range typeFactories {
					if factoryForType == r.factories[index] {
						errs = append(errs, fmt.Errorf(
							"failed to validate '%s' from '%s': %w",
							r.factories[index].name, r.factories[index].source,
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
				factory.name, factory.source, err,
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

		// Skip factories without output type.
		if factory.getOutType() == nil {
			continue
		}

		// Handle the single spawned factory output value.
		outValue := factory.getOutValue()

		// Get the factory result object interface.
		service := outValue.Interface()

		// Close service implementing `Close() error` interface.
		if closer, ok := service.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf(
					"failed to close service '%s' of '%s' from '%s': %w",
					outValue.Type(), factory.name, factory.source, err),
				)
			}
		}

		// Close service implementing `Close()` interface.
		if closer, ok := service.(interface{ Close() }); ok {
			closer.Close()
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
		return reflect.Value{}, fmt.Errorf("%w: '%s'", ErrServiceNotResolved, serviceType)
	}

	// Pick first found service value.
	return resolvedValues[0], nil
}

// resolveByType resolves all service fits to specified type.
func (r *registry) resolveByType(serviceType reflect.Type) ([]reflect.Value, error) {
	// Lookup factory definition by an output service type.
	factories := r.findFactories(serviceType)
	services := make([]reflect.Value, 0, len(factories))

	for _, factory := range factories {
		// Handle found factory definition.
		if err := r.spawnFactory(factory); err != nil {
			return nil, fmt.Errorf("failed to spawn factory '%s': %w", factory.funcType, err)
		}

		// Handle service from factory output (single value).
		services = append(services, factory.getOutValue())
	}

	return services, nil
}

// findFactories lookups for all factories for an output type in the registry.
func (r *registry) findFactories(serviceType reflect.Type) []*factory {
	var factories []*factory

	// Lookup for factories in the registry.
	for _, factory := range r.factories {
		// Skip factories without output type.
		if factory.getOutType() == nil {
			continue
		}

		// Desired service type matched.
		if factory.getOutType() == serviceType {
			factories = append(factories, factory)
			continue
		}

		// Desired service type implements an interface.
		if serviceType.Kind() == reflect.Interface {
			if factory.getOutType().Implements(serviceType) {
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

	// Get or spawn factory input values recursively.
	inValues := make([]reflect.Value, 0, len(factory.inTypes))
	for _, inType := range factory.inTypes {
		// Handle `context.Context` as a special case.
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

	// Call the factory using input arguments.
	outValues := factory.funcValue.Call(inValues)

	// Set factory output values.
	factory.setOutValues(outValues)

	// Handle error returned by the factory.
	if err := factory.getOutError(); err != nil {
		return fmt.Errorf("%w: %w", ErrFactoryReturnedError, err)
	}

	// Set the factory spawn status.
	factory.setIsSpawned(true)

	// Save the factory spawn order.
	r.mutex.Lock()
	r.sequence = append(r.sequence, factory)
	r.mutex.Unlock()

	return nil
}

// isEmptyInterface returns true when argument is an `any` interface.
func isEmptyInterface(typ reflect.Type) bool {
	return typ.Kind() == reflect.Interface && typ.NumMethod() == 0
}

// isContextInterface returns true when argument is a context interface.
func isContextInterface(typ reflect.Type) bool {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(ctxType)
}

// isErrorInterface returns true when argument is an error interface.
func isErrorInterface(typ reflect.Type) bool {
	errType := reflect.TypeOf((*error)(nil)).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(errType)
}
