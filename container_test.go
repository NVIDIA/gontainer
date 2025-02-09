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
	"sync/atomic"
	"testing"
	"time"
)

// TestContainerLifecycle tests container lifecycle.
func TestContainerLifecycle(t *testing.T) {
	factoryStarted := atomic.Bool{}
	serviceStarted := atomic.Bool{}
	serviceClosed := atomic.Bool{}

	svc1 := &testService1{}
	svc2 := &testService2{}
	svc3 := &testService3{}
	svc4 := &testService4{}
	svc5 := testService5(func() error {
		return fmt.Errorf("svc5 error")
	})

	container, err := New(
		NewService(float64(100500)),
		NewFactory(func() string { return "string" }),
		NewFactory(func() (int, int64) { return 123, 456 }),
		NewFactory(func() *testService1 { return svc1 }),
		NewFactory(func() *testService2 { return svc2 }),
		NewFactory(func() (*testService3, *testService4) { return svc3, svc4 }),
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
		) any {
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
			factoryStarted.Store(true)

			// Service function.
			return func() error {
				serviceStarted.Store(true)
				<-ctx.Done()
				serviceClosed.Store(true)
				return nil
			}
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Assert factories and services.
	equal(t, len(container.Factories()), 11)
	equal(t, len(container.Services()), 0)

	// Start all factories in the container.
	equal(t, container.Start(), nil)
	equal(t, factoryStarted.Load(), true)
	equal(t, serviceClosed.Load(), false)

	// Assert factories and services.
	equal(t, len(container.Factories()), 11)
	equal(t, len(container.Services()), 13)

	// Let factory function start executing in the background.
	time.Sleep(time.Millisecond)

	equal(t, serviceStarted.Load(), true)
	equal(t, serviceClosed.Load(), false)

	// Close all factories in the container.
	equal(t, container.Close(), nil)
	equal(t, serviceClosed.Load(), true)

	// Assert context is closed.
	select {
	case <-container.Done():
	default:
		t.Fatalf("context is not closed")
	}

	// Assert factories and services.
	equal(t, len(container.Factories()), 11)
	equal(t, len(container.Services()), 0)
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
