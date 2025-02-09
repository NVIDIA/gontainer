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
	"errors"
	"reflect"
	"testing"
)

// TestEvents tests events broker.
func TestEvents(t *testing.T) {
	testEvent1Args := [][]any(nil)
	testEvent2Args := [][]any(nil)
	testEvent3Args := [][]any(nil)
	testEvent4Args := [][]any(nil)
	testEvent5Args := [][]any(nil)
	testEvent6Args := [][]any(nil)
	testEvent7Args := [][]any(nil)

	ev := &events{events: make(map[string][]handler)}
	ev.Subscribe("TestEvent1", func(args ...any) {
		testEvent1Args = append(testEvent1Args, args)
	})
	ev.Subscribe("TestEvent2", func(args ...any) error {
		testEvent2Args = append(testEvent2Args, args)
		return nil
	})
	ev.Subscribe("TestEvent3", func(x string, y int, z bool) error {
		testEvent3Args = append(testEvent3Args, []any{x, y, z})
		return nil
	})
	ev.Subscribe("TestEvent4", func(x string, y int) error {
		testEvent4Args = append(testEvent4Args, []any{x, y})
		return nil
	})
	ev.Subscribe("TestEvent5", func(x string, y int, z bool) error {
		testEvent5Args = append(testEvent5Args, []any{x, y, z})
		return nil
	})
	ev.Subscribe("TestEvent6", func(err *struct{}) {
		testEvent6Args = append(testEvent6Args, []any{err})
	})
	ev.Subscribe("TestEvent7", func(err error) {
		testEvent7Args = append(testEvent7Args, []any{err})
	})

	equal(t, ev.Trigger(NewEvent("TestEvent1", 1)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent1", "x")), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent2", true)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent3", "x", 1, true)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent4", "x", 1, true)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent5", "x", 1)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent6", nil)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent6", (*struct{})(nil))), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent6", &struct{}{})), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent7", nil)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent7", (error)(nil))), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent7", errors.New("error"))), nil)
	equal(t, testEvent1Args, [][]any{{1}, {"x"}})
	equal(t, testEvent2Args, [][]any{{true}})
	equal(t, testEvent3Args, [][]any{{"x", 1, true}})
	equal(t, testEvent4Args, [][]any{{"x", 1}})
	equal(t, testEvent5Args, [][]any{{"x", 1, false}})
	equal(t, testEvent6Args, [][]any{{(*struct{})(nil)}, {(*struct{})(nil)}, {&struct{}{}}})
	equal(t, testEvent7Args, [][]any{{(error)(nil)}, {(error)(nil)}, {errors.New("error")}})
}

func equal(t *testing.T, a, b any) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("equal failed: '%v' != '%v'", a, b)
	}
}
