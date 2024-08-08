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
		return fmt.Errorf("%w: %w", ErrFactoryRegisterFailed, err)
	}

	// Validate loaded factory object for the registry.
	for _, factoryOutType := range factory.factoryOutTypes {
		// Factories returning `any` may be duplicated.
		if !isEmptyInterface(factoryOutType) {
			// Validate uniqueness of the every factory output type.
			if s, _ := r.findFactoryFor(factoryOutType); s != nil {
				return fmt.Errorf("%w: service duplicate", ErrFactoryRegisterFailed)
			}
		}
	}

	// Register the factory in the registry.
	r.factories = append(r.factories, factory)

	return nil
}

// produceServices spawns all services in the registry.
func (r *registry) produceServices() error {
	for _, factory := range r.factories {
		if err := r.spawnFactory(factory); err != nil {
			return fmt.Errorf(
				"failed to spawn services of %s from '%s': %w",
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
			dependencyZeroValue := reflect.New(serviceType).Elem()
			return getOptionalBox(serviceType, dependencyZeroValue), nil
		}

		// Return an error for non-optional types.
		return reflect.Value{}, fmt.Errorf("%w: '%s'", ErrServiceNotFound, serviceType)
	}

	// Handle found factory definition.
	if err := r.spawnFactory(factory); err != nil {
		return reflect.Value{}, fmt.Errorf("%w: '%s': %w", ErrFactorySpawnFailed, factory.factoryType, err)
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

// findFactoryFor lookups for a factory by an output type in the registry.
func (r *registry) findFactoryFor(serviceType reflect.Type) (*Factory, int) {
	// Lookup for a factory in the registry.
	for _, factory := range r.factories {
		for index, factoryOutType := range factory.factoryOutTypes {
			// If requested type matched.
			if factoryOutType == serviceType {
				return factory, index
			}

			// if requested type implements a non-empty interface.
			if isNonEmptyInterface(serviceType) {
				if factoryOutType.Implements(serviceType) {
					return factory, index
				}
			}
		}
	}

	// Factory definition not found.
	return nil, 0
}

// spawnFactory instantiates specified factory definition.
func (r *registry) spawnFactory(factory *Factory) error {
	// Protect from cyclic dependencies.
	if getStackDepth() >= stackDepthLimit {
		return ErrStackDepthLimit
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
		return fmt.Errorf("failed to invoke factory: %w", err)
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

// ErrFactoryRegisterFailed declares factory load failed error.
var ErrFactoryRegisterFailed = errors.New("factory register failed")

// ErrFactorySpawnFailed declares factory spawn failed error.
var ErrFactorySpawnFailed = errors.New("factory spawn failed")

// ErrServiceNotFound declares service not found error.
var ErrServiceNotFound = errors.New("service not found")

// ErrStackDepthLimit declares a reach of stack limit error.
var ErrStackDepthLimit = errors.New("stack depth limit reached")
