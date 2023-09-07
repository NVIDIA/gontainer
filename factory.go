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
)

// Factory declares service factory.
type Factory struct {
	// Factory function.
	factoryFunc any

	// Factory event handlers.
	factoryEvents map[string][]any

	// Factory is loaded.
	factoryLoaded bool

	// Factory is spawned.
	factorySpawned bool

	// Factory context value.
	factoryCtx context.Context

	// Factory context cancel.
	ctxCancel context.CancelFunc

	// Factory function type.
	factoryType reflect.Type

	// Factory function value.
	factoryValue reflect.Value

	// Factory input types.
	factoryInTypes []reflect.Type

	// Factory output types.
	factoryOutTypes []reflect.Type

	// Factory output error.
	factoryOutError bool

	// Factory result values.
	factoryOutValues []reflect.Value

	// Factory event handlers types.
	factoryEventsTypes map[string][]reflect.Type

	// Factory event handlers values.
	factoryEventsValues map[string][]reflect.Value

	// Factory event handlers input types.
	factoryEventsInTypes map[reflect.Type][]reflect.Type

	// Factory event handlers output errors.
	factoryEventsOutErrors map[reflect.Type]bool
}

// load initializes factory definition internal values.
func (f *Factory) load(ctx context.Context) error {
	if f.factoryLoaded {
		return errors.New("invalid factory func: already loaded")
	}

	// Prepare cancellable context for the factory services.
	f.factoryCtx, f.ctxCancel = context.WithCancel(ctx)

	// Check factory configured.
	if f.factoryFunc == nil {
		return errors.New("invalid factory func: no func specified")
	}

	// Validate factory type and signature.
	f.factoryType = reflect.TypeOf(f.factoryFunc)
	f.factoryValue = reflect.ValueOf(f.factoryFunc)
	if f.factoryType.Kind() != reflect.Func {
		return fmt.Errorf("invalid factory func: not a function: %s", f.factoryType)
	}

	// Index factory input types from the function signature.
	f.factoryInTypes = make([]reflect.Type, 0, f.factoryType.NumIn())
	for index := 0; index < f.factoryType.NumIn(); index++ {
		f.factoryInTypes = append(f.factoryInTypes, f.factoryType.In(index))
	}

	// Index factory output types from the function signature.
	f.factoryOutTypes = make([]reflect.Type, 0, f.factoryType.NumOut())
	f.factoryOutValues = make([]reflect.Value, 0, f.factoryType.NumOut())
	for index := 0; index < f.factoryType.NumOut(); index++ {
		if index != f.factoryType.NumOut()-1 || f.factoryType.Out(index) != errorType {
			// Register regular factory output type.
			f.factoryOutTypes = append(f.factoryOutTypes, f.factoryType.Out(index))
		} else {
			// Register last output index as an error.
			f.factoryOutError = true
		}
	}

	// Prepare factory-level event handlers.
	f.factoryEventsTypes = map[string][]reflect.Type{}
	f.factoryEventsValues = map[string][]reflect.Value{}
	f.factoryEventsInTypes = map[reflect.Type][]reflect.Type{}
	f.factoryEventsOutErrors = map[reflect.Type]bool{}
	for event, handlers := range f.factoryEvents {
		for _, handler := range handlers {
			// Get handler function type and value.
			handlerType := reflect.TypeOf(handler)
			handlerValue := reflect.ValueOf(handler)

			// Check that handler in a function.
			if handlerType.Kind() != reflect.Func {
				return fmt.Errorf("invalid handler func: not a function: %s", handlerType)
			}

			// Check that handler function returns nothing or an error.
			switch {
			case handlerType.NumOut() == 0:
			case handlerType.NumOut() == 1 && handlerType.Out(0) == errorType:
			default:
				return fmt.Errorf("invalid handler func: bad signature: %s", handlerType)
			}

			// Collect input types for the handler function.
			handlerInTypes := make([]reflect.Type, 0, handlerType.NumIn())
			for index := 0; index < handlerType.NumIn(); index++ {
				handlerInTypes = append(handlerInTypes, handlerType.In(index))
			}

			f.factoryEventsTypes[event] = append(f.factoryEventsTypes[event], handlerType)
			f.factoryEventsValues[event] = append(f.factoryEventsValues[event], handlerValue)
			f.factoryEventsInTypes[handlerType] = handlerInTypes
			f.factoryEventsOutErrors[handlerType] = handlerType.NumOut() == 1
		}
	}

	// Save the factory load status.
	f.factoryLoaded = true
	return nil
}

// FactoryOpt defines factory option.
type FactoryOpt func(*Factory)

// NewService creates new service factory with predefined service.
func NewService[T any](singleton T) *Factory {
	return &Factory{
		factoryFunc:   func() T { return singleton },
		factoryEvents: map[string][]any{},
	}
}

// NewFactory creates new service factory with factory func.
func NewFactory(factoryFn any, opts ...FactoryOpt) *Factory {
	factory := &Factory{
		factoryFunc:   factoryFn,
		factoryEvents: map[string][]any{},
	}
	for _, opt := range opts {
		opt(factory)
	}
	return factory
}

// WithSubscribe registers event handler for the factory.
func WithSubscribe(event string, handler any) FactoryOpt {
	return func(factory *Factory) {
		factory.factoryEvents[event] = append(factory.factoryEvents[event], handler)
	}
}
