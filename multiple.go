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

// Multiple defines a dependency on zero or more services of the same type.
//
// This generic wrapper is used in service factory function parameters to declare
// a dependency on all services assignable to type T registered in the container.
//
// The container will collect and inject all matching services into the slice.
// For interface types, multiple matches are allowed.
// For concrete (non-interface) types, at most one match is possible.
//
// Example:
//
//	func MyFactory(providers gontainer.Multiple[AuthProvider]) {
//	    for _, p := range providers {
//	        ...
//	    }
//	}
type Multiple[T any] []T

// multipleElem reports the element type T. Inside the instantiated method T is
// known statically, so it is read directly from the type parameter rather than
// reverse-engineered from the slice's element type.
func (m Multiple[T]) multipleElem() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// multipleBox is the internal contract implemented only by Multiple[T]. The
// value receiver lets isMultipleType detect the box from a plain reflect.Zero
// value without any pointer indirection.
type multipleBox interface {
	multipleElem() reflect.Type
}

// isMultipleType checks and returns multiple box type.
func isMultipleType(typ reflect.Type) (reflect.Type, bool) {
	// Check if the type is a slice.
	if typ.Kind() != reflect.Slice {
		return nil, false
	}

	// Check if the type is a Multiple type.
	box, ok := reflect.Zero(typ).Interface().(multipleBox)
	if !ok {
		return nil, false
	}

	return box.multipleElem(), true
}

// newMultipleValue packs multiple values to the slice.
func newMultipleValue(typ reflect.Type, values []reflect.Value) reflect.Value {
	return reflect.Append(reflect.Zero(typ), values...)
}
