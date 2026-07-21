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
	"testing"
)

// TestIsMultipleType tests checking of argument to be multiple.
func TestIsMultipleType(t *testing.T) {
	var t1 any
	var t2 string
	var t3 Multiple[int]

	typ := reflect.TypeOf(&t1).Elem()
	rtyp, ok := isMultipleType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t2).Elem()
	rtyp, ok = isMultipleType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t3).Elem()
	rtyp, ok = isMultipleType(typ)
	equal(t, rtyp, reflect.TypeOf((*int)(nil)).Elem())
	equal(t, ok, true)
}

// TestIsMultipleTypePointer verifies that a *Multiple[T] parameter type is
// rejected cleanly rather than panicking. reflect.Zero of a pointer is a nil
// pointer whose method set still includes Multiple's value receiver, so a bare
// marker-interface assertion would call multipleElem() on nil and dereference it.
func TestIsMultipleTypePointer(t *testing.T) {
	// DEFENSIVE (not a real use case): *Multiple[T] is not a legitimate parameter
	// type - Multiple[T] is taken by value. This only pins that such input degrades
	// to a clean dependency error instead of panicking, as it did pre-refactor.
	typ := reflect.TypeOf((*Multiple[int])(nil)) // *Multiple[int]

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("isMultipleType panicked on *Multiple[int]: %v", r)
		}
	}()

	rtyp, ok := isMultipleType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)
}

// TestIsMultipleTypeEmbedded verifies that a user struct embedding Multiple[T]
// is not misdetected as a multiple box.
func TestIsMultipleTypeEmbedded(t *testing.T) {
	// embedsMultiple inherits all embedded type methods.
	type embedsMultiple struct {
		Multiple[int]
	}

	typ := reflect.TypeOf(embedsMultiple{})
	rtyp, ok := isMultipleType(typ)
	equal(t, ok, false)
	equal(t, rtyp, nil)
}

// TestNewMultipleValue tests creation of multiple value.
func TestNewMultipleValue(t *testing.T) {
	// When multiple not found.
	box := Multiple[string]{}
	value := newMultipleValue(reflect.TypeOf(box), nil)
	equal(t, value.Interface().(Multiple[string]), Multiple[string](nil))

	// When multiple found.
	box = Multiple[string]{}
	data := []reflect.Value{reflect.ValueOf("result1"), reflect.ValueOf("result2")}
	value = newMultipleValue(reflect.TypeOf(box), data)
	equal(t, value.Interface().(Multiple[string]), Multiple[string]{"result1", "result2"})
}
