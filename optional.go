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

// optionalElem reports the wrapped type T. Inside the instantiated method T is
// known statically, so it is read directly from the type parameter rather than
// reverse-engineered from a struct field.
func (o Optional[T]) optionalElem() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// withValue returns a present optional box carrying v. T is known inside the
// instantiated method, so v is unwrapped with a plain type assertion - no
// reflection-based mutation of an unexported field is needed.
func (o Optional[T]) withValue(v reflect.Value) any {
	return Optional[T]{value: v.Interface().(T), ok: true}
}

// optionalBox is the internal contract implemented only by Optional[T]. Both
// methods use value receivers, so an instantiated Optional[T] satisfies it
// without any pointer indirection - this is what lets isOptionalType detect the
// box from a plain reflect.Zero value and build one without addressability.
type optionalBox interface {
	optionalElem() reflect.Type
	withValue(v reflect.Value) any
}

// isOptionalType checks and returns optional box type.
func isOptionalType(typ reflect.Type) (reflect.Type, bool) {
	box, ok := reflect.Zero(typ).Interface().(optionalBox)
	if !ok {
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
