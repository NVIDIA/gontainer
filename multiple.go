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

// isMultipleType checks and returns multiple box type.
func isMultipleType(typ reflect.Type) (reflect.Type, bool) {
	// Check if the type is a slice.
	if typ.Kind() != reflect.Slice {
		return nil, false
	}

	// Check if the type is a Multiple type.
	sample := reflect.TypeOf(Multiple[struct{}]{})
	if typ.PkgPath() != sample.PkgPath() {
		return nil, false
	}

	// Check if the type is a Multiple type.
	sampleName := sample.Name()
	sep := strings.IndexByte(sampleName, '[')
	if sep < 0 || !strings.HasPrefix(typ.Name(), sampleName[:sep+1]) {
		return nil, false
	}

	// Return the element type of the slice.
	return typ.Elem(), true
}

// newMultipleValue packs multiple values to the slice.
func newMultipleValue(typ reflect.Type, values []reflect.Value) reflect.Value {
	box := reflect.New(typ).Elem()
	return reflect.Append(box, values...)
}
