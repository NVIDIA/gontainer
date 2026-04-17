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
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
)

// TestContainer tests service container.
func TestContainer(t *testing.T) {
	svc1 := &testService1{}
	svc2 := &testService2{}
	svc3 := &testService3{}
	svc4 := &testService4{}
	svc5 := testService5(func() error {
		return fmt.Errorf("svc5 error")
	})

	// Prepare started flag.
	started := atomic.Bool{}
	closed := atomic.Bool{}
	invoked := atomic.Bool{}

	// Run container.
	equal(t, Run(
		NewService(float64(100500)),
		NewFactory(func() string { return "string" }),
		NewFactory(func() int { return 123 }),
		NewFactory(func() int64 { return 456 }),
		NewFactory(func() *testService1 { return svc1 }),
		NewFactory(func() *testService2 { return svc2 }),
		NewFactory(func() *testService3 { return svc3 }),
		NewFactory(func() *testService4 { return svc4 }),
		NewFactory(func() testService5 { return svc5 }),
		NewFactory(func() (float32, func() error) {
			started.Store(true)
			return 123, func() error {
				closed.Store(true)
				return nil
			}
		}),
		NewEntrypoint(func(
			dep1 float64,
			dep2 string,
			dep3 Optional[int],
			dep4 Optional[bool],
			dep5 Multiple[interface{ Do2() }],
			dep6 testService5,
			dep7 interface{ Do5() error },
			dep8 Optional[testService5],
			dep9 Optional[interface{ Do5() error }],
			dep10 Optional[func() error],
			dep11 float32,
		) {
			equal(t, dep1, float64(100500))
			equal(t, dep2, "string")
			equal(t, dep3.Get(), 123)
			equal(t, dep4.Get(), false)
			equal(t, dep5, Multiple[interface{ Do2() }]{svc1, svc2})
			equal(t, dep6().Error(), "svc5 error")
			equal(t, dep6.Do5().Error(), "svc5 error")
			equal(t, dep7.Do5().Error(), "svc5 error")
			equal(t, dep8.Get()().Error(), "svc5 error")
			equal(t, dep8.Get().Do5().Error(), "svc5 error")
			equal(t, dep9.Get().Do5().Error(), "svc5 error")
			equal(t, dep10.Get(), (func() error)(nil))
			equal(t, dep11, float32(123))
			invoked.Store(true)
		}),
	), nil)

	// Assert flags are set.
	equal(t, started.Load(), true)
	equal(t, closed.Load(), true)
	equal(t, invoked.Load(), true)
}

type testService1 struct{}

func (t *testService1) Do1() {}
func (t *testService1) Do2() {}
func (t *testService1) Do3() {}

type testService2 struct{}

func (t *testService2) Do1() {}
func (t *testService2) Do2() {}

type testService3 struct{}

func (t *testService3) Do1() {}

type testService4 struct{}

func (t *testService4) Do1() {}

type testService5 func() error

func (t testService5) Do5() error { return t() }

func equal(t *testing.T, a, b any) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("equal failed: '%v' != '%v'", a, b)
	}
}

// TestDistinctTypes pins down the container's type-matching contract for
// named types that share a common underlying type (e.g. `type UsersDB *sql.DB`
// and `type OrdersDB *sql.DB`). Resolution is based on exact type identity:
// there is no covariance between a defined type and its underlying type,
// so users can register multiple instances of the "same" underlying type
// by wrapping each one in a distinct named type.
func TestDistinctTypes(t *testing.T) {
	t.Run("DistinctDefinedTypesCoexist", func(t *testing.T) {
		// Two defined pointer types share the same underlying `*connection`,
		// yet the container treats them as fully independent service keys:
		// both are registered without a duplicate-type error and each is
		// injected into the entrypoint from its own factory.
		type connection struct{ id string }
		type usersDB *connection
		type ordersDB *connection

		var users usersDB
		var orders ordersDB

		equal(t, Run(
			NewFactory(func() usersDB { return &connection{id: "users"} }),
			NewFactory(func() ordersDB { return &connection{id: "orders"} }),
			NewEntrypoint(func(u usersDB, o ordersDB) {
				users = u
				orders = o
			}),
		), nil)

		equal(t, (*connection)(users).id, "users")
		equal(t, (*connection)(orders).id, "orders")
	})

	t.Run("NoCovarianceBetweenDefinedAndUnderlying", func(t *testing.T) {
		// A factory that returns a defined type does not satisfy a request
		// for the underlying type: the container uses exact type matching,
		// not assignability or convertibility.
		type connection struct{ id string }
		type usersDB *connection

		err := Run(
			NewFactory(func() usersDB { return &connection{id: "users"} }),
			NewEntrypoint(func(_ *connection) {}),
		)
		equal(t, errors.Is(err, ErrDependencyNotResolved), true)
	})

	t.Run("MultipleDoesNotCollectDefinedTypes", func(t *testing.T) {
		// Multiple[T] is polymorphic only for interfaces. For concrete types
		// it follows the same exact-match rule, so it does not pick up
		// factories of defined types that happen to share the same underlying.
		type connection struct{ id string }
		type usersDB *connection
		type ordersDB *connection

		var all Multiple[*connection]

		equal(t, Run(
			NewFactory(func() usersDB { return &connection{id: "users"} }),
			NewFactory(func() ordersDB { return &connection{id: "orders"} }),
			NewEntrypoint(func(m Multiple[*connection]) { all = m }),
		), nil)

		equal(t, len(all), 0)
	})
}
