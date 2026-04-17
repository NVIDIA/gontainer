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
	"reflect"
	"strings"
)

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
func (o *Optional[T]) Get() T {
	return o.value
}

// Ok reports whether the optional service was provided by the container.
func (o *Optional[T]) Ok() bool {
	return o.ok
}

// setValue populates the private value field.
func (o *Optional[T]) setValue(v reflect.Value) {
	reflect.ValueOf(&o.value).Elem().Set(v)
	o.ok = true
}

// isOptionalType checks and returns optional box type.
func isOptionalType(typ reflect.Type) (reflect.Type, bool) {
	// Check if the type is a struct.
	if typ.Kind() != reflect.Struct {
		return nil, false
	}

	// Check if the type is a Optional type.
	sample := reflect.TypeOf(Optional[struct{}]{})
	if typ.PkgPath() != sample.PkgPath() {
		return nil, false
	}

	// Check if the type is a Optional type.
	sampleName := sample.Name()
	sep := strings.IndexByte(sampleName, '[')
	if sep < 0 || !strings.HasPrefix(typ.Name(), sampleName[:sep+1]) {
		return nil, false
	}

	// Check if the type has a value field.
	field, ok := typ.FieldByName("value")
	if !ok {
		return nil, false
	}

	// Return the type of the value field.
	return field.Type, true
}

// newOptionalValue creates new optional type with a value.
func newOptionalValue(typ reflect.Type, value reflect.Value) reflect.Value {
	// Allocate an addressable pointer to a zero Optional[T].
	ptr := reflect.New(typ)

	// Populate the private field via the internal setter interface.
	ptr.Interface().(interface{ setValue(reflect.Value) }).setValue(value)

	return ptr.Elem()
}

// newOptionalZero creates a new optional type with no value and ok set to false.
func newOptionalZero(typ reflect.Type) reflect.Value {
	return reflect.New(typ).Elem()
}
