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
)

// registry contains all defined factories metadata.
type registry struct {
	factories []*Factory
	sequence  []*Factory
	events    Events
}

// registerFactory registers factory in the registry.
func (r *registry) registerFactory(factory *Factory) error {
	// Load the factory definition.
	if err := factory.load(); err != nil {
		return fmt.Errorf("failed to load factory: %w", err)
	}

	// Register the factory in the registry.
	r.factories = append(r.factories, factory)

	return nil
}

// validateFactories validates registered factories.
// Validate checks for availability of all non-optional types
// and for possible circular dependencies between factories.
func (r *registry) validateFactories() error {
	var errs []error

	// Validate all input types are resolvable.
	for _, factory := range r.factories {
		for index, factoryInType := range factory.factoryInTypes {
			// Is this type a special factory context type?
			if isContextInterface(factoryInType) {
				continue
			}

			// Is this type wrapped to the `Optional[type]`?
			_, isOptional := isOptionalType(factoryInType)
			if isOptional {
				continue
			}

			// Is this type wrapped to the `Multiple[type]`?
			_, isMultiple := isMultipleType(factoryInType)
			if isMultiple {
				continue
			}

			// Is a factory for this type could be resolved?
			typeFactories, _ := r.findFactoriesFor(factoryInType)
			if len(typeFactories) == 0 {
				errs = append(errs, fmt.Errorf(
					"failed to validate argument '%s' (index %d) of factory '%s' from '%s': %w",
					factoryInType, index, factory.Name(), factory.Source(), ErrServiceNotResolved,
				))
				continue
			}
		}
	}

	// Validate all output types are unique.
	for _, factory := range r.factories {
		for index, factoryOutType := range factory.factoryOutTypes {
			// Factories returning `any` could be duplicated.
			if isEmptyInterface(factoryOutType) {
				continue
			}

			// Validate uniqueness of the every factory output type.
			factoriesForSameOutType, _ := r.findFactoriesFor(factoryOutType)
			if len(factoriesForSameOutType) > 1 {
				errs = append(errs, fmt.Errorf(
					"failed to validate output '%s' (index %d) of factory '%s' from '%s': %w",
					factoryOutType, index, factory.Name(), factory.Source(), ErrServiceDuplicated,
				))
			}
		}
	}

	// Validate for circular dependencies.
	for index := range r.factories {
		factories := []*Factory{r.factories[index]}
	recursion:
		for len(factories) > 0 {
			factory := factories[0]
			factories = factories[1:]

			for _, factoryInType := range factory.factoryInTypes {
				// Is this type a special factory context type?
				if isContextInterface(factoryInType) {
					continue
				}

				// Is this type wrapped to the `Optional[type]`?
				innerType, isOptional := isOptionalType(factoryInType)
				if isOptional {
					factoryInType = innerType
				}

				// Is this type wrapped to the `Multiple[type]`?
				innerType, isMultiple := isMultipleType(factoryInType)
				if isMultiple {
					factoryInType = innerType
				}

				// Walk through all factories for this in argument type.
				factoriesForType, _ := r.findFactoriesFor(factoryInType)
				for _, factoryForType := range factoriesForType {
					if factoryForType == r.factories[index] {
						errs = append(errs, fmt.Errorf(
							"failed to validate factory '%s' from '%s': %w",
							r.factories[index].Name(), r.factories[index].Source(), ErrCircularDependency,
						))
						break recursion
					}
				}

				factories = append(factories, factoriesForType...)
			}
		}
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// spawnFactories spawns all factories in the registry.
func (r *registry) spawnFactories() error {
	for _, factory := range r.factories {
		if err := r.spawnFactory(factory); err != nil {
			return fmt.Errorf(
				"failed to spawn services of '%s' from '%s': %w",
				factory.Name(), factory.Source(), err,
			)
		}
	}

	return nil
}

// closeFactories closes all factories in the reverse order.
func (r *registry) closeFactories() error {
	var errs []error
	for index := len(r.sequence) - 1; index >= 0; index-- {
		factory := r.sequence[index]
		factory.ctxCancel()

		// Handle every factory output value.
		for _, factoryOutValue := range factory.factoryOutValues {
			if factoryOutValue.IsValid() {
				// Get the factory result object interface.
				service := factoryOutValue.Interface()

				// Close service implementing `Close() error` interface.
				// Service functions will wait for the function return.
				if closer, ok := service.(interface{ Close() error }); ok {
					if err := closer.Close(); err != nil {
						errs = append(errs, fmt.Errorf(
							"%s from '%s': %w",
							factory.Name(), factory.Source(), err),
						)
					}
				}

				// Close service implementing `Close()` interface.
				if closer, ok := service.(interface{ Close() }); ok {
					closer.Close()
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close services: %w", errors.Join(errs...))
	}

	return nil
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
	factories, factoryOutIndexes := r.findFactoriesFor(serviceType)
	services := make([]reflect.Value, 0, len(factories))

	for index, factory := range factories {
		// Handle found factory definition.
		if err := r.spawnFactory(factory); err != nil {
			return nil, fmt.Errorf("failed to spawn factory '%s': %w", factory.factoryType, err)
		}

		// Handle services from factory outputs.
		for _, factoryOutIndex := range factoryOutIndexes[index] {
			services = append(services, factory.factoryOutValues[factoryOutIndex])
		}
	}

	return services, nil
}

// findFactoryFor lookups for all factories for an output type in the registry.
func (r *registry) findFactoriesFor(serviceType reflect.Type) ([]*Factory, [][]int) {
	var factories []*Factory
	var outputs [][]int

	// Lookup for factories in the registry.
	for _, factory := range r.factories {
		var factoryOutputs []int
		for index, factoryOutType := range factory.factoryOutTypes {
			// If requested type matched.
			if factoryOutType == serviceType {
				factories = append(factories, factory)
				factoryOutputs = append(factoryOutputs, index)
				continue
			}

			// if requested type implements a non-empty interface.
			if isNonEmptyInterface(serviceType) {
				if factoryOutType.Implements(serviceType) {
					factories = append(factories, factory)
					factoryOutputs = append(factoryOutputs, index)
					continue
				}
			}
		}
		if len(factoryOutputs) > 0 {
			outputs = append(outputs, factoryOutputs)
		}
	}

	// Return matched factories and indices.
	return factories, outputs
}

// spawnFactory instantiates specified factory definition.
func (r *registry) spawnFactory(factory *Factory) error {
	// Protect from cyclic dependencies.
	if getStackDepth() >= stackDepthLimit {
		return ErrStackLimitReached
	}

	// Check factory already spawned.
	if factory.factorySpawned {
		return nil
	}

	// Get or spawn factory input values recursively.
	factoryInValues := make([]reflect.Value, 0, len(factory.factoryInTypes))
	for _, factoryInType := range factory.factoryInTypes {
		// Handle specified `context.Context` as a special case.
		if isContextInterface(factoryInType) {
			factoryCtxValue := reflect.ValueOf(factory.factoryCtx)
			factoryInValues = append(factoryInValues, factoryCtxValue)
			continue
		}

		// Resolve factory input dependency.
		factoryInValue, err := r.resolveService(factoryInType)
		if err != nil {
			return fmt.Errorf("failed to resolve service: %w", err)
		}

		factoryInValues = append(factoryInValues, factoryInValue)
	}

	// Spawn the factory using the factory input arguments.
	factoryOutValues := factory.factoryValue.Call(factoryInValues)
	factoryErrorValue := reflect.Value{}

	// Read optional factory error value.
	if factory.factoryOutError {
		factoryErrorValue = factoryOutValues[len(factoryOutValues)-1]
		factoryOutValues = factoryOutValues[0 : len(factoryOutValues)-1]
	}

	// Handle factory output error if present.
	if factory.factoryOutError && !factoryErrorValue.IsNil() {
		err, _ := factoryErrorValue.Interface().(error)
		return fmt.Errorf("%w: %w", ErrFactoryReturnedError, err)
	}

	// Handle factory out functions as regular objects.
	for factoryOutIndex, factoryOutValue := range factoryOutValues {
		if serviceFuncValue, ok := isServiceFunc(factoryOutValue); ok {
			serviceFuncResult, err := startServiceFunc(serviceFuncValue)
			if err != nil {
				return fmt.Errorf("failed to start factory func: %w", err)
			}
			factoryOutValues[factoryOutIndex] = serviceFuncResult
		}
	}

	// Register factory output values in the registry.
	factory.factoryOutValues = factoryOutValues

	// Recorder services spawn order.
	r.sequence = append(r.sequence, factory)

	// Save the factory spawn status.
	factory.factorySpawned = true
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

// isContextInterface returns true when argument is a context interface.
func isContextInterface(typ reflect.Type) bool {
	var ctx context.Context
	ctxType := reflect.TypeOf(&ctx).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(ctxType)
}

// isServiceFunc returns true when the argument is a service function.
// The `factoryOutValue` can be an `any` type with a function in the value
// (when the factory declares `any` return type), or the `function` type
// (when the factory declares explicit return type).
func isServiceFunc(factoryOutValue reflect.Value) (reflect.Value, bool) {
	// Unbox the value if it is an any interface.
	if isEmptyInterface(factoryOutValue.Type()) {
		factoryOutValue = factoryOutValue.Elem()
	}

	// Check if the result value kind is a func kind.
	// The func type must have no user-defined methods,
	// because otherwise it could be a service instance
	// based on the func type but implements an interface.
	if factoryOutValue.Kind() == reflect.Func {
		if factoryOutValue.NumMethod() == 0 {
			return factoryOutValue, true
		}
	}

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
