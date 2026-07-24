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

// TestIsOptionalTypePointer tests that a *Optional[T] type is rejected without a panic.
func TestIsOptionalTypePointer(t *testing.T) {
	typ := reflect.TypeOf((*Optional[int])(nil))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("isOptionalType panicked on *Optional[int]: %v", r)
		}
	}()

	rtyp, ok := isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)
}

// TestIsOptionalTypeEmbedded tests that a user struct embedding Optional[T] is
// not misdetected as an optional box.
func TestIsOptionalTypeEmbedded(t *testing.T) {
	// embedsOptional inherits all embedded type methods.
	type embedsOptional struct {
		Optional[int]
		Extra int
	}

	typ := reflect.TypeOf(embedsOptional{})

	// The embedder satisfies optionalBox through promotion.
	_, satisfies := reflect.Zero(typ).Interface().(optionalBox)
	equal(t, satisfies, true)

	rtyp, ok := isOptionalType(typ)
	equal(t, ok, false)
	equal(t, rtyp, nil)
}

// TestNewOptionalValueNilInterface tests that a service resolving to a nil
// interface value is boxed as a present optional (nil value, ok true).
func TestNewOptionalValueNilInterface(t *testing.T) {
	// optNilIface exercises a service that resolves to a nil interface value.
	type optNilIface interface{ marker() }

	var svc optNilIface
	data := reflect.ValueOf(&svc).Elem()

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

// TestOptionalValueSemantics tests that Get and Ok are callable on
// non-addressable values, i.e. that they use value receivers.
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

// optService is a sample service used by the constructor tests.
type optService struct{ id int }

// TestOptionalZeroValueAbsent tests that the natural zero value of Optional[T]
// represents an absent value.
func TestOptionalZeroValueAbsent(t *testing.T) {
	var optional Optional[*optService]
	equal(t, optional.Ok(), false)
	equal(t, optional.Get(), (*optService)(nil))
}

// TestNewOptional tests that NewOptional creates a present value carrying an
// ordinary value.
func TestNewOptional(t *testing.T) {
	present := NewOptional(42)
	equal(t, present.Ok(), true)
	equal(t, present.Get(), 42)
}

// TestNewOptionalZeroValue tests that NewOptional creates a present value even
// when the wrapped value is the zero value of a value type.
func TestNewOptionalZeroValue(t *testing.T) {
	presentZero := NewOptional(0)
	equal(t, presentZero.Ok(), true)
	equal(t, presentZero.Get(), 0)
}

// TestNewOptionalNil tests that NewOptional[*T](nil) creates a present value
// that carries nil, rather than an absent value.
func TestNewOptionalNil(t *testing.T) {
	presentNil := NewOptional[*optService](nil)
	equal(t, presentNil.Ok(), true)
	equal(t, presentNil.Get(), (*optService)(nil))
}

// TestNewOptionalFactoryCall tests that an Optional built by NewOptional is
// usable when a factory function is called directly.
func TestNewOptionalFactoryCall(t *testing.T) {
	// factory consumes an optional dependency the same way a container would.
	factory := func(dep Optional[*optService]) (bool, *optService) {
		return dep.Ok(), dep.Get()
	}

	svc := &optService{id: 7}
	ok, got := factory(NewOptional(svc))
	equal(t, ok, true)
	equal(t, got, svc)

	// A present nil dependency is still reported as present.
	ok, got = factory(NewOptional[*optService](nil))
	equal(t, ok, true)
	equal(t, got, (*optService)(nil))

	// A zero-value Optional is reported as absent.
	ok, got = factory(Optional[*optService]{})
	equal(t, ok, false)
	equal(t, got, (*optService)(nil))
}
