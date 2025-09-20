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
	"context"
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

	started := atomic.Bool{}
	container, err := New(
		context.Background(),
		NewService(float64(100500)),
		NewFactory(func() string { return "string" }),
		NewFactory(func() int { return 123 }),
		NewFactory(func() int64 { return 456 }),
		NewFactory(func() *testService1 { return svc1 }),
		NewFactory(func() *testService2 { return svc2 }),
		NewFactory(func() *testService3 { return svc3 }),
		NewFactory(func() *testService4 { return svc4 }),
		NewFactory(func() testService5 { return svc5 }),
		NewFactory(func(
			ctx context.Context,
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
		) float32 {
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
			started.Store(true)
			return 123
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)
	equal(t, len(container.Factories()), 12)
	equal(t, started.Load(), false)

	// Start all factories in the container.
	equal(t, container.Start(), nil)
	equal(t, started.Load(), true)

	// Close all factories in the container.
	equal(t, container.Close(), nil)

	// Assert context is closed.
	select {
	case <-container.Done():
	default:
		t.Fatalf("context is not closed")
	}
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

type testServiceWithClose struct {
	ctx    context.Context
	closed *atomic.Bool
}

func (t *testServiceWithClose) Close() error {
	t.closed.Store(true)
	return nil
}

func equal(t *testing.T, a, b any) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("equal failed: '%v' != '%v'", a, b)
	}
}
