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
	"regexp"
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

// normalizeSourceLines strips every "    at <file>:<line>" meta line
// from a rendered error so tests can assert the structural shape of
// Traceback / Source blocks without pinning brittle file paths or
// line numbers.
func normalizeSourceLines(s string) string {
	sourceLineRegex := regexp.MustCompile(`\n {4}at [^\n]+`)
	return sourceLineRegex.ReplaceAllString(s, "")
}

// TestLifecycleCleanupOnFactoryError verifies that cleanup runs when a factory
// returns an error and that the primary error is preserved alongside the run.
func TestLifecycleCleanupOnFactoryError(t *testing.T) {
	type serviceA struct{}

	// Track whether the cleanup callback was invoked.
	cleaned := atomic.Bool{}
	factoryErr := errors.New("factory boom")

	// Run a container whose only factory fails after providing a cleanup.
	err := Run(
		NewFactory(func() (*serviceA, func() error, error) {
			return &serviceA{}, func() error {
				cleaned.Store(true)
				return nil
			}, factoryErr
		}),
		NewEntrypoint(func(*serviceA) {}),
	)

	// The cleanup must run and the factory error must be reported.
	equal(t, cleaned.Load(), true)
	equal(t, errors.Is(err, factoryErr), true)
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestLifecycleCleanupOnEntrypointError verifies that cleanup runs when the
// entrypoint returns an error and that both errors are preserved.
func TestLifecycleCleanupOnEntrypointError(t *testing.T) {
	type serviceA struct{}

	// Track whether the cleanup callback was invoked.
	cleaned := atomic.Bool{}
	entrypointErr := errors.New("entrypoint boom")

	// Run a container whose entrypoint fails after the service is acquired.
	err := Run(
		NewFactory(func() (*serviceA, func() error) {
			return &serviceA{}, func() error {
				cleaned.Store(true)
				return nil
			}
		}),
		NewEntrypoint(func(*serviceA) error { return entrypointErr }),
	)

	// The cleanup must run and the entrypoint error must be reported.
	equal(t, cleaned.Load(), true)
	equal(t, errors.Is(err, entrypointErr), true)
	equal(t, errors.Is(err, ErrEntrypointReturnedError), true)
}

// TestLifecycleCleanupReverseOrder verifies that cleanup callbacks run in the
// reverse of their acquisition order.
func TestLifecycleCleanupReverseOrder(t *testing.T) {
	type serviceA struct{}
	type serviceB struct{}
	type serviceC struct{}

	// Record the order in which cleanups run; closeFactories is sequential.
	var order []string

	// Chain three services so that A is acquired first and C last.
	err := Run(
		NewFactory(func() (*serviceA, func() error) {
			return &serviceA{}, func() error { order = append(order, "A"); return nil }
		}),
		NewFactory(func(*serviceA) (*serviceB, func() error) {
			return &serviceB{}, func() error { order = append(order, "B"); return nil }
		}),
		NewFactory(func(*serviceB) (*serviceC, func() error) {
			return &serviceC{}, func() error { order = append(order, "C"); return nil }
		}),
		NewEntrypoint(func(*serviceC) {}),
	)

	// Cleanups must unwind in reverse acquisition order.
	equal(t, err, nil)
	equal(t, order, []string{"C", "B", "A"})
}

// TestLifecycleCleanupErrorFactoryRunsBeforeDependencies verifies that a factory
// returning both a cleanup and an error has its cleanup invoked before its
// dependencies, and that the service value returned with the error is not exposed.
func TestLifecycleCleanupErrorFactoryRunsBeforeDependencies(t *testing.T) {
	type dependency struct{}
	type failing struct{ id string }

	// Record cleanup order and capture any value injected into the entrypoint.
	var order []string
	var injected *failing
	factoryErr := errors.New("failing factory boom")

	// The failing factory depends on another service that also provides cleanup.
	err := Run(
		NewFactory(func() (*dependency, func() error) {
			return &dependency{}, func() error { order = append(order, "dependency"); return nil }
		}),
		NewFactory(func(*dependency) (*failing, func() error, error) {
			return &failing{id: "leaked"}, func() error { order = append(order, "failing"); return nil }, factoryErr
		}),
		NewEntrypoint(func(f *failing) { injected = f }),
	)

	// The failing factory's cleanup must precede its dependency's cleanup.
	equal(t, errors.Is(err, factoryErr), true)
	equal(t, order, []string{"failing", "dependency"})

	// The service returned together with the error must never be injected.
	equal(t, injected, (*failing)(nil))
}

// TestLifecycleMultipleCleanupErrors verifies that every cleanup error is
// preserved together via errors.Join.
func TestLifecycleMultipleCleanupErrors(t *testing.T) {
	type serviceA struct{}
	type serviceB struct{}

	errA := errors.New("cleanup A failed")
	errB := errors.New("cleanup B failed")

	// Both services fail during cleanup while execution otherwise succeeds.
	err := Run(
		NewFactory(func() (*serviceA, func() error) {
			return &serviceA{}, func() error { return errA }
		}),
		NewFactory(func(*serviceA) (*serviceB, func() error) {
			return &serviceB{}, func() error { return errB }
		}),
		NewEntrypoint(func(*serviceB) {}),
	)

	// Both cleanup errors must be reachable on the joined error.
	equal(t, err != nil, true)
	equal(t, errors.Is(err, errA), true)
	equal(t, errors.Is(err, errB), true)
}

// TestLifecycleCleanupContinuesAfterError verifies that a failing cleanup does
// not prevent the remaining cleanup callbacks from running.
func TestLifecycleCleanupContinuesAfterError(t *testing.T) {
	type serviceA struct{}
	type serviceB struct{}
	type serviceC struct{}

	// Track the cleanups on both sides of the failing one.
	firstCleaned := atomic.Bool{}
	lastCleaned := atomic.Bool{}
	middleErr := errors.New("middle cleanup failed")

	// The middle service fails to clean up; the outer ones must still run.
	err := Run(
		NewFactory(func() (*serviceA, func() error) {
			return &serviceA{}, func() error { firstCleaned.Store(true); return nil }
		}),
		NewFactory(func(*serviceA) (*serviceB, func() error) {
			return &serviceB{}, func() error { return middleErr }
		}),
		NewFactory(func(*serviceB) (*serviceC, func() error) {
			return &serviceC{}, func() error { lastCleaned.Store(true); return nil }
		}),
		NewEntrypoint(func(*serviceC) {}),
	)

	// Cleanups before and after the failing one must both have run.
	equal(t, errors.Is(err, middleErr), true)
	equal(t, firstCleaned.Load(), true)
	equal(t, lastCleaned.Load(), true)
}

// TestLifecycleCleanupRunsExactlyOnce verifies that no cleanup callback is
// invoked more than once, even when a factory and a cleanup both fail.
func TestLifecycleCleanupRunsExactlyOnce(t *testing.T) {
	type serviceA struct{}
	type serviceB struct{}

	// Count how many times each cleanup runs.
	var countA, countB atomic.Int32
	factoryErr := errors.New("factory boom")

	// serviceB fails with a cleanup and an error; serviceA cleanup also fails.
	err := Run(
		NewFactory(func() (*serviceA, func() error) {
			return &serviceA{}, func() error { countA.Add(1); return errors.New("A close failed") }
		}),
		NewFactory(func(*serviceA) (*serviceB, func() error, error) {
			return &serviceB{}, func() error { countB.Add(1); return nil }, factoryErr
		}),
		NewEntrypoint(func(*serviceB) {}),
	)

	// Each cleanup must run exactly once despite the errors.
	equal(t, errors.Is(err, factoryErr), true)
	equal(t, countA.Load(), int32(1))
	equal(t, countB.Load(), int32(1))
}
