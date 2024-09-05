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
	"unsafe"
)

// Optional defines optional service dependency.
type Optional[T any] struct {
	value T
}

// Get returns optional service instance.
func (o Optional[T]) Get() T {
	return o.value
}

// Optional marks this type as optional.
func (o Optional[T]) Optional() {}

// isOptionalType checks and returns optional box type.
func isOptionalType(typ reflect.Type) (reflect.Type, bool) {
	if typ.Kind() == reflect.Struct {
		if _, ok := typ.MethodByName("Optional"); ok {
			if methodValue, ok := typ.MethodByName("Get"); ok {
				if methodValue.Type.NumOut() == 1 {
					methodType := methodValue.Type.Out(0)
					return methodType, true
				}
			}
		}
	}
	return nil, false
}

// newOptionalValue creates new optional type with a value.
func newOptionalValue(typ reflect.Type, value reflect.Value) reflect.Value {
	// Prepare boxing struct for value.
	box := reflect.New(typ).Elem()

	// Inject factory output value to the boxing struct.
	field := box.FieldByName("value")
	pointer := unsafe.Pointer(field.UnsafeAddr())
	public := reflect.NewAt(field.Type(), pointer)
	public.Elem().Set(value)

	return box
}
