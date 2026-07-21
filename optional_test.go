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

// TestIsOptionalType tests checking of argument to be optional.
func TestIsOptionalType(t *testing.T) {
	var t1 any
	var t2 string
	var t3 Optional[int]

	typ := reflect.TypeOf(&t1).Elem()
	rtyp, ok := isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t2).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t3).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, reflect.TypeOf((*int)(nil)).Elem())
	equal(t, ok, true)
}

// TestIsOptionalTypePointer verifies that a *Optional[T] parameter type is
// rejected cleanly (regular dependency) rather than panicking. reflect.Zero of a
// pointer is a nil pointer whose method set still includes Optional's value
// receivers, so a bare marker-interface assertion would call optionalElem() on
// nil and dereference it.
func TestIsOptionalTypePointer(t *testing.T) {
	// DEFENSIVE (not a real use case): *Optional[T] is not a legitimate parameter
	// type - Optional[T] is taken by value. This only pins that such input degrades
	// to a clean dependency error instead of panicking, as it did pre-refactor.
	typ := reflect.TypeOf((*Optional[int])(nil)) // *Optional[int]

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("isOptionalType panicked on *Optional[int]: %v", r)
		}
	}()

	rtyp, ok := isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)
}

// TestIsOptionalTypeEmbedded verifies that a user struct embedding Optional[T]
// is not misdetected as an optional box. The embedded field promotes the marker
// methods, so a bare interface assertion would match it and later build the
// wrong (inner) type.
func TestIsOptionalTypeEmbedded(t *testing.T) {
	// embedsOptional is a user-defined struct that embeds Optional[T]. It is NOT an
	// optional box: it merely promotes Optional's marker methods. The container must
	// treat it as a regular dependency, not misdetect it as a box.
	type embedsOptional struct {
		Optional[int]
		Extra int
	}

	// DEFENSIVE (not a real use case): embedding Optional[T] in a struct is not how
	// the API is used. This only pins that such input is not misdetected as a box
	// (which would later build the wrong type), matching pre-refactor behaviour.
	typ := reflect.TypeOf(embedsOptional{})

	rtyp, ok := isOptionalType(typ)
	equal(t, ok, false)
	equal(t, rtyp, nil)
}

// TestNewOptionalValueNilInterface verifies that a service that resolves to a
// nil interface value is boxed as a present optional (value nil, ok true) rather
// than panicking. The old setValue path used reflect.Set, which accepts nil; a
// v.Interface().(T) assertion panics because the interface is nil.
func TestNewOptionalValueNilInterface(t *testing.T) {
	// optNilIface is an interface used to exercise a service that resolves to a
	// legitimately nil interface value.
	type optNilIface interface{ marker() }

	var svc optNilIface                  // nil interface
	data := reflect.ValueOf(&svc).Elem() // reflect.Value of interface type, nil

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newOptionalValue panicked on nil interface service: %v", r)
		}
	}()

	value := newOptionalValue(reflect.TypeOf(Optional[optNilIface]{}), data)
	opt := value.Interface().(Optional[optNilIface])
	equal(t, opt.Ok(), true)
	equal(t, opt.Get(), nil)
}

// TestNewOptionalValue tests creation of optional value.
func TestNewOptionalValue(t *testing.T) {
	// When optional not found.
	box := Optional[string]{}
	data := reflect.New(reflect.TypeOf((*string)(nil)).Elem()).Elem()
	value := newOptionalValue(reflect.TypeOf(box), data)
	opt := value.Interface().(Optional[string])
	equal(t, opt.Get(), "")

	// When optional found.
	box = Optional[string]{}
	data = reflect.ValueOf("result")
	value = newOptionalValue(reflect.TypeOf(box), data)
	opt = value.Interface().(Optional[string])
	equal(t, opt.Get(), "result")
}

// TestOptionalOkNotProvided tests that Ok returns false when the service is not provided.
func TestOptionalOkNotProvided(t *testing.T) {
	typ := reflect.TypeOf(Optional[string]{})
	value := newOptionalZero(typ)
	opt := value.Interface().(Optional[string])
	if opt.Ok() {
		t.Errorf("expected Ok() to return false for zero optional, got true")
	}
	if opt.Get() != "" {
		t.Errorf("expected Get() to return zero value, got %q", opt.Get())
	}
}

// TestOptionalValueSemantics verifies that Get and Ok are callable on
// non-addressable values (e.g. a map element), i.e. that they use value
// receivers. With pointer receivers this would not compile.
func TestOptionalValueSemantics(t *testing.T) {
	boxes := map[string]Optional[string]{
		"present": newOptionalValue(
			reflect.TypeOf(Optional[string]{}),
			reflect.ValueOf("hi"),
		).Interface().(Optional[string]),
		"absent": {},
	}

	equal(t, boxes["present"].Ok(), true)
	equal(t, boxes["present"].Get(), "hi")
	equal(t, boxes["absent"].Ok(), false)
	equal(t, boxes["absent"].Get(), "")
}

// TestOptionalOkProvided tests that Ok returns true when the service is provided.
func TestOptionalOkProvided(t *testing.T) {
	typ := reflect.TypeOf(Optional[string]{})
	value := newOptionalValue(typ, reflect.ValueOf("hello"))
	opt := value.Interface().(Optional[string])
	if !opt.Ok() {
		t.Errorf("expected Ok() to return true for provided optional, got false")
	}
	if opt.Get() != "hello" {
		t.Errorf("expected Get() to return %q, got %q", "hello", opt.Get())
	}
}
