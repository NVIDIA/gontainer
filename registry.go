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
	"strings"
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
		return fmt.Errorf("factory load: %w", err)
	}

	// Validate loaded factory object for the registry.
	for _, factoryOutType := range factory.factoryOutTypes {
		// Factories returning `any` may be duplicated.
		if !isEmptyInterface(factoryOutType) {
			// Validate uniqueness of the every factory output type.
			if s, _ := r.findFactoryByOutType(factoryOutType); s != nil {
				return fmt.Errorf("factory output duplicate: %s", factoryOutType)
			}
		}
	}

	// Register factory-level event handlers.
	for event, handlerTypes := range factory.factoryEventsTypes {
		for index, handlerType := range handlerTypes {
			handlerValue := factory.factoryEventsValues[event][index]

			// Register handler in events broker.
			r.events.Subscribe(event, func() error {
				// Get or spawn event handler dependencies recursively.
				handlerInTypes := factory.factoryEventsInTypes[handlerType]
				handlerInValues, err := r.getSpawnedFactoryIns(factory.factoryCtx, handlerInTypes)
				if err != nil {
					return err
				}

				// Invoke the handler with resolved dependencies.
				handlerResults := handlerValue.Call(handlerInValues)
				handlerOutError := factory.factoryEventsOutErrors[handlerType]
				if handlerOutError && !handlerResults[0].IsNil() {
					return handlerResults[0].Interface().(error)
				}

				return nil
			})
		}
	}

	// Register the factory in the registry.
	r.factories = append(r.factories, factory)

	return nil
}

// startFactories spawns all factories in the registry.
func (r *registry) startFactories() error {
	for _, factory := range r.factories {
		if err := r.spawnFactory(factory); err != nil {
			return fmt.Errorf("failed to spawn factory '%s': %w", factory.factoryType, err)
		}
	}

	return nil
}

// closeFactories closes services of factories in the reverse order.
func (r *registry) closeFactories() error {
	var errs []string
	for index := len(r.sequence) - 1; index >= 0; index-- {
		factory := r.sequence[index]
		factory.ctxCancel()

		// Handle every factory output value.
		for _, factoryOutValue := range factory.factoryOutValues {
			if factoryOutValue.IsValid() {
				// Get the factory result object interface.
				service := factoryOutValue.Interface()

				// Close service implementing closer interface.
				// Service functions will wait for the function return.
				if closer, ok := service.(interface{ Close() error }); ok {
					if err := closer.Close(); err != nil {
						errs = append(errs, err.Error())
					}
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close services: %s", strings.Join(errs, "; "))
	}

	return nil
}

// getSpawnedFactoryIns spawns, boxes and returns specified factory input types.
func (r *registry) getSpawnedFactoryIns(ctx context.Context, factoryInTypes []reflect.Type) ([]reflect.Value, error) {
	factoryInValues := make([]reflect.Value, 0, len(factoryInTypes))
	for _, origFactoryInType := range factoryInTypes {
		// Handle specified `context.Context` as a special case.
		if isContextInterface(origFactoryInType) {
			factoryInValues = append(factoryInValues, reflect.ValueOf(ctx))
			continue
		}

		// Check that optional factory input dependency was specified.
		factoryInType, inIsOptional := checkIsOptionalIn(origFactoryInType)

		// Lookup factory dependency factory definition.
		factory, factoryOutIndex := r.findFactoryByOutType(factoryInType)

		// Factory dependency definition may be omitted by the registry state.
		// For example, if factory accepts `gontainer.Optional[Config]`, then
		// the `Config` type service factory may be provided and may be omitted.
		if factory == nil && !inIsOptional {
			return nil, fmt.Errorf("failed to get factory for type '%s'", factoryInType)
		}

		// Handle found factory definition.
		if factory != nil {
			if err := r.spawnFactory(factory); err != nil {
				return nil, fmt.Errorf("failed to spawn factory '%s': %w", factory.factoryType, err)
			}
		}

		// Optional factory input values should be boxed before passing them to factory.
		// For example, if factory accepts `gontainer.Optional[Config]`, then it is
		// required to wrap the `config` service (or nil) with the `Optional` struct.
		if inIsOptional {
			// Box optional dependency factory output value that may be empty.
			factoryInValue := boxFactoryOptionalIn(origFactoryInType, factory, factoryOutIndex)
			factoryInValues = append(factoryInValues, factoryInValue)
		} else {
			// Use direct dependency factory output value.
			factoryInValue := factory.factoryOutValues[factoryOutIndex]
			factoryInValues = append(factoryInValues, factoryInValue)
		}
	}

	return factoryInValues, nil
}

// findFactoryByOutType lookups for a factory by an output type in the registry.
func (r *registry) findFactoryByOutType(serviceType reflect.Type) (*Factory, int) {
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
	// Check factory already spawned.
	if factory.factorySpawned {
		return nil
	}

	// Get or spawn factory input values recursively.
	factoryInValues, err := r.getSpawnedFactoryIns(factory.factoryCtx, factory.factoryInTypes)
	if err != nil {
		return fmt.Errorf("failed to get factory input values: %w", err)
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
		return fmt.Errorf("failed to call factory: %w", err)
	}

	// Handle factory out functions as regular objects.
	for factoryOutIndex, factoryOutValue := range factoryOutValues {
		factoryOutValues[factoryOutIndex], err = boxFactoryOutFunc(factoryOutValue)
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
	var ctxType = reflect.TypeOf(&ctx).Elem()
	return typ.Kind() == reflect.Interface && typ.Implements(ctxType)
}

// checkIsOptionalIn checks and returns optional input type.
func checkIsOptionalIn(typ reflect.Type) (reflect.Type, bool) {
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

// boxFactoryOptionalIn boxes an optional factory input to structs.
func boxFactoryOptionalIn(typ reflect.Type, factory *Factory, outIndex int) reflect.Value {
	// Prepare boxing struct for value.
	box := reflect.New(typ).Elem()

	// Inject factory output value to the boxing struct.
	if factory != nil {
		field := box.FieldByName("value")
		pointer := unsafe.Pointer(field.UnsafeAddr())
		public := reflect.NewAt(field.Type(), pointer)
		public.Elem().Set(factory.factoryOutValues[outIndex])
	}

	return box
}

// boxFactoryOutFunc boxes specified function to the factory out regular object.
func boxFactoryOutFunc(factoryOutValue reflect.Value) (reflect.Value, error) {
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
