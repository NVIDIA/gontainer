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
	"slices"
	"unsafe"
)

// registry contains all defined factories metadata.
type registry struct {
	factories []*Factory
	sequence  []*Factory
	events    Events
}

// registerFactory registers factory in the registry.
func (r *registry) registerFactory(ctx context.Context, factory *Factory) error {
	// Load the factory definition.
	if err := factory.load(ctx); err != nil {
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

	// Validate all factories.
	for _, factory := range r.factories {
		// Validate all input types are resolvable.
		for index, factoryInType := range factory.factoryInTypes {
			// Is this type a special factory context type?
			if isContextInterface(factoryInType) {
				continue
			}

			// Is this type wrapped to the `Optional[type]`?
			_, isOptionalType := isOptionalBoxType(factoryInType)
			if isOptionalType {
				continue
			}

			// Is a factory for this type could be resolved?
			factoryInTypeFactory, _ := r.findFactoryFor(factoryInType)
			if factoryInTypeFactory == nil {
				errs = append(errs, fmt.Errorf(
					"failed to validate service '%s' (argument %d) of '%s' from '%s': %w",
					factoryInType, index, factory.Name(), factory.Source(), ErrServiceNotResolved,
				))
				continue
			}
		}

		// Validate all input types have no circular dependencies.
		for index, factoryInType := range factory.factoryInTypes {
			// Is this type a special factory context type?
			if isContextInterface(factoryInType) {
				continue
			}

			// Validate dependencies graph of this type.
			validationQueue := []reflect.Type{factoryInType}
			var validatedTypesTypes []reflect.Type
			for len(validationQueue) > 0 {
				validatingType := validationQueue[0]
				validationQueue = validationQueue[1:]

				// Is this type wrapped to the `Optional[type]`?
				validatingType, _ = isOptionalBoxType(validatingType)

				// Is a factory for this type could be resolved?
				nextTypeFactory, _ := r.findFactoryFor(validatingType)
				if nextTypeFactory == nil {
					continue
				}

				// Was this type already validated before?
				if slices.Contains(validatedTypesTypes, validatingType) {
					errs = append(errs, fmt.Errorf(
						"failed to validate service '%s' (argument %d) of '%s' from '%s': %w",
						validatingType, index, factory.Name(), factory.Source(), ErrCircularDependency,
					))
					break
				}

				// Register type validation step.
				validationQueue = append(validationQueue, nextTypeFactory.factoryInTypes...)
				validatedTypesTypes = append(validatedTypesTypes, validatingType)
			}
		}

		// Validate all output types are unique.
		for index, factoryOutType := range factory.factoryOutTypes {
			// Factories returning `any` could be duplicated.
			if isEmptyInterface(factoryOutType) {
				continue
			}

			// Validate uniqueness of the every factory output type.
			factoriesForSameOutType, _ := r.findFactoriesFor(factoryOutType)
			if len(factoriesForSameOutType) > 1 {
				errs = append(errs, fmt.Errorf(
					"failed to validate service '%s' (output %d) of '%s' from '%s': %w",
					factoryOutType, index, factory.Name(), factory.Source(), ErrServiceDuplicated,
				))
			}
		}
	}

	// Join all found errors.
	return errors.Join(errs...)
}

// produceServices spawns all services in the registry.
func (r *registry) produceServices() error {
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

// closeServices closes all services in the reverse order.
func (r *registry) closeServices() error {
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
	// Detect whether the dependency type is wrapped to the `Optional[type]` box.
	// If yes, resolve the type wrapped to the optional wrapper type.
	realServiceType, isOptional := isOptionalBoxType(serviceType)

	// Lookup factory definition by an output service type.
	factory, factoryOutIndex := r.findFactoryFor(realServiceType)

	// Resolve of regular types leads to an error if the factory for the type is not registered
	// in the container. Resolve of optional types works differently: if the optional type is
	// not found, then a zero-value box should be returned instead. For example, resolving an
	// unregistered type `Config` triggers an error, while resolving `gontainer.Optional[Config]`
	// returns a zero-value box.
	if factory == nil {
		// Return a zero optional value.
		if isOptional {
			dependencyZeroValue := reflect.New(realServiceType).Elem()
			return getOptionalBox(serviceType, dependencyZeroValue), nil
		}

		// Return an error for non-optional types.
		return reflect.Value{}, fmt.Errorf("%w: '%s'", ErrServiceNotResolved, serviceType)
	}

	// Handle found factory definition.
	if err := r.spawnFactory(factory); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to spawn factory '%s': %w", factory.factoryType, err)
	}

	// Get resolved service value.
	serviceValue := factory.factoryOutValues[factoryOutIndex]

	// Return resolved service in an optional box type.
	// For example, if a factory accepts `gontainer.Optional[Config]`,
	// then it is required to wrap the `Config` with an `Optional` struct.
	if isOptional {
		return getOptionalBox(serviceType, serviceValue), nil
	}

	// Return resolved dependency value.
	return serviceValue, nil
}

// findFactoryFor lookups for all factories for an output type in the registry.
func (r *registry) findFactoriesFor(serviceType reflect.Type) ([]*Factory, []int) {
	var factories []*Factory
	var outputs []int

	// Lookup for factories in the registry.
	for _, factory := range r.factories {
		for index, factoryOutType := range factory.factoryOutTypes {
			// If requested type matched.
			if factoryOutType == serviceType {
				factories = append(factories, factory)
				outputs = append(outputs, index)
				continue
			}

			// if requested type implements a non-empty interface.
			if isNonEmptyInterface(serviceType) {
				if factoryOutType.Implements(serviceType) {
					factories = append(factories, factory)
					outputs = append(outputs, index)
					continue
				}
			}
		}
	}

	// Return matched factories and indices.
	return factories, outputs
}

// findFactoryFor lookups for a factory for an output type in the registry.
func (r *registry) findFactoryFor(serviceType reflect.Type) (*Factory, int) {
	// Search for factories by the output type.
	factories, outputs := r.findFactoriesFor(serviceType)
	if len(factories) == 0 {
		return nil, 0
	}

	// Return found factory and out index.
	return factories[0], outputs[0]
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
		var err error
		factoryOutValues[factoryOutIndex], err = wrapFactoryFunc(factoryOutValue)
		if err != nil {
			return fmt.Errorf("failed to wrap factory func: %w", err)
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

// function represents service function return value wrapper.
type function chan error

// Close awaits service function and returns result error.
func (f function) Close() error {
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

// isOptionalBoxType checks and returns optional box type.
func isOptionalBoxType(typ reflect.Type) (reflect.Type, bool) {
	if typ.Kind() == reflect.Struct {
		if methodValue, ok := typ.MethodByName("Get"); ok {
			if methodValue.Type.NumOut() == 1 {
				methodType := methodValue.Type.Out(0)
				return methodType, true
			}
		}
	}
	return typ, false
}

// getOptionalBox boxes an optional factory input to structs.
func getOptionalBox(typ reflect.Type, value reflect.Value) reflect.Value {
	// Prepare boxing struct for value.
	box := reflect.New(typ).Elem()

	// Inject factory output value to the boxing struct.
	field := box.FieldByName("value")
	pointer := unsafe.Pointer(field.UnsafeAddr())
	public := reflect.NewAt(field.Type(), pointer)
	public.Elem().Set(value)

	return box
}

// wrapFactoryFunc wraps specified function to the regular service object.
func wrapFactoryFunc(factoryOutValue reflect.Value) (reflect.Value, error) {
	// Check specified factory out value elem is a function.
	// The factoryOutValue can be an `any` type with a function in the value,
	// when the factory declares any return type, or directly a function type,
	// when the factory declares explicit func return type. Both cases are OK.
	if factoryOutValue.Kind() == reflect.Interface {
		if factoryOutValue.Elem().Kind() == reflect.Func {
			factoryOutValue = factoryOutValue.Elem()
		}
	}
	if factoryOutValue.Kind() != reflect.Func {
		return factoryOutValue, nil
	}

	// Check specified factory out value is a service function.
	// It is a programming error, if the function has wrong interface.
	factoryOutServiceFn, ok := factoryOutValue.Interface().(func() error)
	if !ok {
		return factoryOutValue, fmt.Errorf("unexpected signature '%s'", factoryOutValue.Elem().Type())
	}

	// Prepare a regular object from the function.
	fnResult := function(make(chan error))

	// Run specified function in background and await for return.
	go func() { fnResult <- factoryOutServiceFn() }()

	// Return reflected value of the wrapper.
	return reflect.ValueOf(fnResult), nil
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

// ErrFactoryReturnedError declares factory returned an error.
var ErrFactoryReturnedError = errors.New("factory returned error")

// ErrServiceDuplicated declares service duplicated error.
var ErrServiceDuplicated = errors.New("service duplicated")

// ErrServiceNotResolved declares service not resolved error.
var ErrServiceNotResolved = errors.New("service not resolved")

// ErrCircularDependency declares a cyclic dependency error.
var ErrCircularDependency = errors.New("circular dependency")

// ErrStackLimitReached declares a reach of stack limit error.
var ErrStackLimitReached = errors.New("stack limit reached")
