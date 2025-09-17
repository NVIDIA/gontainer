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

// Events is a lightweight event broker
// used for decoupled communication between services within the container.
//
// It supports two types of event handlers:
//   - handlers with a variadic signature: `func(args ...any) [error]`;
//   - handlers with a typed signature: `func(T1, T2, ...) [error]`.
//
// Handlers may optionally return an error, which will be collected and joined
// during the Trigger phase. All handlers for a given event are executed synchronously.
type Events struct {
	mutex  sync.RWMutex
	events map[string][]handler
}

// Subscribe registers an event handler for the specified event name.
//
// The handler must be a function with one of the following signatures:
//   - `func(args ...any) [error]`;
//   - `func(T1, T2, ...) [error]`.
//
// If the handler returns an error, it will be captured when the event is triggered.
// Panics if the handler is not a function or has an unsupported signature.
func (em *Events) Subscribe(name string, handlerFn any) {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	// Validate event handler type.
	handlerValue := reflect.ValueOf(handlerFn)
	handlerType := handlerValue.Type()
	if handlerType.Kind() != reflect.Func {
		panic(fmt.Sprintf("unexpected event handler type: %T", handlerFn))
	}

	// Validate event handler output signature.
	switch {
	case handlerType.NumOut() == 0:
	case handlerType.NumOut() == 1 && handlerType.Out(0).Implements(errorType):
	default:
		panic(fmt.Sprintf("unexpected event handler signature: %T", handlerFn))
	}

	// Register event handler function.
	if handlerType.NumIn() == 1 && handlerType.In(0) == anySliceType {
		// Register a function that accepts a variable number of any arguments.
		em.events[name] = append(em.events[name], func(event *Event) error {
			return em.callAnyVarHandler(handlerValue, event.Args())
		})
	} else {
		// Register a function that accepts concrete argument types.
		em.events[name] = append(em.events[name], func(event *Event) error {
			return em.callTypedHandler(handlerValue, event.Args())
		})
	}
}

// Trigger dispatches the given event to all registered handlers.
//
// Handlers are called synchronously in the order they were registered.
// All returned errors are collected and joined into a single error.
func (em *Events) Trigger(event *Event) error {
	em.mutex.RLock()
	defer em.mutex.RUnlock()

	errs := make([]error, 0, len(em.events[event.Name()]))
	for _, handler := range em.events[event.Name()] {
		if err := handler(event); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// callTypedHandler calls `func(TypeA, TypeB, TypeC) [error]` event handler.
func (em *Events) callTypedHandler(handler reflect.Value, args []any) error {
	// Prepare slice of in arguments for handler.
	handlerInArgs := make([]reflect.Value, 0, handler.Type().NumIn())

	// Fill handler args with provided event args.
	maxArgsLen := min(len(args), handler.Type().NumIn())
	for index := 0; index < maxArgsLen; index++ {
		eventArgValue := reflect.ValueOf(args[index])
		handlerArgType := handler.Type().In(index)

		// Convert untyped nil values to typed nils (zero value for pointer types).
		if !eventArgValue.IsValid() && isNillableType(handlerArgType) {
			eventArgValue = reflect.New(handlerArgType).Elem()
		}

		// Allow to pass only values which are not untyped nils.
		if !eventArgValue.IsValid() {
			return fmt.Errorf(
				"%w: argument '%s' could not reveive type 'nil' (index %d)",
				ErrHandlerArgTypeMismatch, handlerArgType, index,
			)
		}

		// Allow to pass only assignable to handler arg type values.
		if !eventArgValue.Type().AssignableTo(handlerArgType) {
			return fmt.Errorf(
				"%w: argument '%s' could not reveive type '%s' (index %d)",
				ErrHandlerArgTypeMismatch, handlerArgType, eventArgValue.Type(), index,
			)
		}

		handlerInArgs = append(handlerInArgs, eventArgValue)
	}

	// Fill handler args with default type values.
	if len(handlerInArgs) < handler.Type().NumIn() {
		for index := len(handlerInArgs); index < handler.Type().NumIn(); index++ {
			zeroValuePtr := reflect.New(handler.Type().In(index))
			handlerInArgs = append(handlerInArgs, zeroValuePtr.Elem())
		}
	}

	// Invoke original event handler function.
	handlerOutArgs := handler.Call(handlerInArgs)
	return em.getCallOutError(handlerOutArgs)
}

// callAnyVarHandler calls `func(...any) [error]` event handler.
func (em *Events) callAnyVarHandler(handler reflect.Value, args []any) error {
	// Prepare slice of in arguments for handler.
	handlerInArgs := make([]reflect.Value, 0, len(args))
	for _, arg := range args {
		handlerInArgs = append(handlerInArgs, reflect.ValueOf(arg))
	}

	// Invoke original event handler function.
	handlerOutArgs := handler.Call(handlerInArgs)
	return em.getCallOutError(handlerOutArgs)
}

func (em *Events) getCallOutError(outArgs []reflect.Value) error {
	if len(outArgs) == 1 {
		// Use the value as an error.
		// Ignore failed cast of nil error.
		err, _ := outArgs[0].Interface().(error)
		return err
	}

	return nil
}

// Event declares service container events.
type Event struct {
	name string
	args []any
}

// Name returns event name.
func (e *Event) Name() string { return e.name }

// Args returns event arguments.
func (e *Event) Args() []any { return e.args }

// NewEvent returns new event instance.
func NewEvent(name string, args ...any) *Event {
	return &Event{name: name, args: args}
}

// handler declares event handler function.
type handler func(event *Event) error

// anySliceType contains reflection type for any slice variable.
var anySliceType = reflect.TypeOf((*[]any)(nil)).Elem()

// isNillableType returns true whether the specified type kind could accept nil.
func isNillableType(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Chan, reflect.Interface:
		return true
	default:
		return false
	}
}

// ErrHandlerArgTypeMismatch declares handler argument type mismatch error.
var ErrHandlerArgTypeMismatch = errors.New("handler argument type mismatch")
