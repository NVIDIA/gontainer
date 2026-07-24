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

import "reflect"

// Optional defines a dependency on a service that may or may not be registered.
//
// This generic wrapper is used in service factory function parameters to declare
// that the service of type T is optional. If the container does not contain
// a matching service, the zero value of T will be injected.
//
// Use the Get() method to access the wrapped value inside the factory.
//
// Example:
//
//	func MyFactory(logger gontainer.Optional[Logger]) {
//	    if log := logger.Get(); log != nil {
//	        log.Info("Logger available")
//	    }
//	}
type Optional[T any] struct {
	value T
	ok    bool
}

// Get returns the optional service instance.
func (o Optional[T]) Get() T {
	return o.value
}

// Ok reports whether the optional service was provided by the container.
func (o Optional[T]) Ok() bool {
	return o.ok
}

// optionalSelf reports Optional's own instantiated type, used to reject types
// that merely embed Optional[T] and promote its methods.
func (o Optional[T]) optionalSelf() reflect.Type {
	return reflect.TypeOf(o)
}

// optionalElem reports the wrapped type T.
func (o Optional[T]) optionalElem() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// withValue returns a present optional box carrying v. The comma-ok assertion
// keeps a nil interface value as the zero T with ok set, instead of panicking.
func (o Optional[T]) withValue(v reflect.Value) any {
	value, _ := v.Interface().(T)
	return Optional[T]{value: value, ok: true}
}

// optionalBox is the internal contract implemented only by Optional[T].
type optionalBox interface {
	optionalSelf() reflect.Type
	optionalElem() reflect.Type
	withValue(v reflect.Value) any
}

// isOptionalType checks and returns optional box type.
func isOptionalType(typ reflect.Type) (reflect.Type, bool) {
	// Check if the type is a struct.
	if typ.Kind() != reflect.Struct {
		return nil, false
	}

	// Check if the type is an Optional type, rejecting structs that only embed it.
	box, ok := reflect.Zero(typ).Interface().(optionalBox)
	if !ok || box.optionalSelf() != typ {
		return nil, false
	}

	return box.optionalElem(), true
}

// newOptionalValue creates new optional type with a value.
func newOptionalValue(typ reflect.Type, value reflect.Value) reflect.Value {
	box := reflect.Zero(typ).Interface().(optionalBox)
	return reflect.ValueOf(box.withValue(value))
}

// newOptionalZero creates a new optional type with no value and ok set to false.
func newOptionalZero(typ reflect.Type) reflect.Value {
	return reflect.New(typ).Elem()
}

// NewOptional creates a present Optional that carries the given value.
//
// It is primarily meant for testing factory functions in isolation: a factory
// that accepts an Optional[T] parameter can be called directly with a value
// built here, without standing up a container.
//
// The constructor always creates a present value: Ok reports true even when
// value is the zero value of its type or a nil pointer, map, slice, channel,
// function or interface. A present value may therefore be nil, and a nil value
// passed here represents a present nil rather than an absent value.
//
// The absence of a value is represented by the zero value of Optional[T], for
// which Ok reports false:
//
//	var absent Optional[*Service]         // absent, Ok() == false
//	present := NewOptional[*Service](nil) // present nil, Ok() == true
func NewOptional[T any](value T) Optional[T] {
	return Optional[T]{value: value, ok: true}
}
