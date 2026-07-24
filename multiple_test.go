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

// TestIsMultipleTypePointer tests that a *Multiple[T] type is rejected without a panic.
func TestIsMultipleTypePointer(t *testing.T) {
	typ := reflect.TypeOf((*Multiple[int])(nil))

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

	// The embedder satisfies multipleBox through promotion.
	_, satisfies := reflect.Zero(typ).Interface().(multipleBox)
	equal(t, satisfies, true)

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

// mulService is a sample service used by the constructor tests.
type mulService struct{ id int }

// TestNewMultipleEmpty tests that calling NewMultiple without arguments creates
// a valid empty collection.
func TestNewMultipleEmpty(t *testing.T) {
	empty := NewMultiple[*mulService]()
	equal(t, len(empty), 0)
}

// TestNewMultipleSingle tests that NewMultiple creates a collection holding a
// single provided value.
func TestNewMultipleSingle(t *testing.T) {
	serviceA := &mulService{id: 1}
	multiple := NewMultiple(serviceA)
	equal(t, multiple, Multiple[*mulService]{serviceA})
}

// TestNewMultipleValues tests that NewMultiple creates a collection holding all
// provided values.
func TestNewMultipleValues(t *testing.T) {
	serviceA := &mulService{id: 1}
	serviceB := &mulService{id: 2}
	multiple := NewMultiple(serviceA, serviceB)
	equal(t, multiple, Multiple[*mulService]{serviceA, serviceB})
}

// TestNewMultipleOrder tests that NewMultiple preserves the order of the values.
func TestNewMultipleOrder(t *testing.T) {
	multiple := NewMultiple(3, 1, 2)
	equal(t, multiple, Multiple[int]{3, 1, 2})
}

// TestNewMultipleCopiesInput tests that NewMultiple copies its input and does
// not alias a slice expanded at the call site.
func TestNewMultipleCopiesInput(t *testing.T) {
	source := []int{1, 2, 3}
	multiple := NewMultiple(source...)

	// Mutating the source must not affect the constructed collection.
	source[0] = 99
	equal(t, multiple, Multiple[int]{1, 2, 3})
}

// TestNewMultipleFactoryCall tests that a Multiple built by NewMultiple is
// usable when a factory function is called directly.
func TestNewMultipleFactoryCall(t *testing.T) {
	// factory consumes a multiple dependency the same way a container would.
	factory := func(deps Multiple[*mulService]) []int {
		ids := make([]int, 0, len(deps))
		for _, dep := range deps {
			ids = append(ids, dep.id)
		}
		return ids
	}

	serviceA := &mulService{id: 1}
	serviceB := &mulService{id: 2}
	equal(t, factory(NewMultiple(serviceA, serviceB)), []int{1, 2})

	// An empty collection yields no ids.
	equal(t, factory(NewMultiple[*mulService]()), []int{})
}
