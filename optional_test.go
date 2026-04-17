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
