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
