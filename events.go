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
)

// Events declaration.
const (
	// ContainerClose declares container close intention.
	ContainerClose = "ContainerClose"

	// UnhandledPanic declares unhandled panic in container.
	UnhandledPanic = "UnhandledPanic"
)

// Events declares event broker type.
type Events interface {
	// Subscribe registers event handler.
	Subscribe(name string, handler any)

	// Trigger triggers specified event handlers.
	Trigger(event Event) error
}

// events implements Events interface.
type events map[string][]Handler

// Subscribe registers event handler.
func (e events) Subscribe(name string, handler any) {
	var handlerWrapper Handler

	// Infer event handler signature.
	switch handler := handler.(type) {
	case func():
		handlerWrapper = func(...any) error { handler(); return nil }
	case func(...any):
		handlerWrapper = func(args ...any) error { handler(args...); return nil }
	case func() error:
		handlerWrapper = func(...any) error { return handler() }
	case func(...any) error:
		handlerWrapper = func(args ...any) error { return handler(args...) }
	default:
		panic(fmt.Sprintf("unexpected event handler type: %T", handler))
	}

	e[name] = append(e[name], handlerWrapper)
}

// Trigger triggers specified event handlers.
func (e events) Trigger(event Event) error {
	errs := make([]error, 0, len(e[event.Name()]))
	for _, handler := range e[event.Name()] {
		if err := handler(event.Args()...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Event declares service container events.
type Event interface {
	// Name returns event name.
	Name() string

	// Args returns event arguments.
	Args() []any
}

// NewEvent returns new event instance.
func NewEvent(name string, args ...any) Event {
	return event{name: name, args: args}
}

// Handler declares event handler function.
type Handler func(args ...any) error

// event wraps string event.
type event struct {
	name string
	args []any
}

// Name implements Event interface.
func (e event) Name() string { return e.name }

// Args implements Event interface.
func (e event) Args() []any { return e.args }
