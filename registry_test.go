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
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRegistryRegisterFactory tests corresponding registry method.
func TestRegistryRegisterFactory(t *testing.T) {
	fun := func(a, b, c string) (int, error) {
		return 1, nil
	}

	option := NewFactory(fun)
	registry := &registry{}
	equal(t, option.apply(registry), nil)
	factory := registry.factories[0]
	equal(t, factory.funcValue.IsValid(), true)
	equal(t, factory.funcValue.Kind(), reflect.Func)
}

// TestRegistryValidateFactories tests corresponding registry method.
func TestRegistryValidateFactories(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr func(t *testing.T, err error)
	}{
		{
			name: "NoValidationErrors",
			options: []Option{
				NewFactory(func(bool) (int, error) { return 1, nil }),
				NewFactory(func(string) (bool, error) { return true, nil }),
				NewFactory(func() (string, error) { return "s", nil }),
				NewEntrypoint(func(int, bool, string) {}),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, err, nil)
			},
		},
		{
			name: "ServiceNotResolvedError",
			options: []Option{
				NewFactory(func(bool) (int, error) { return 0, nil }),
				NewFactory(func(string) (int32, error) { return 0, nil }),
				NewEntrypoint(func(int, int32) {}),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrDependencyNotResolved), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 2)

				equal(t, errors.Is(errs[0], ErrDependencyNotResolved), true)
				equal(t, normalizeSourceLines(errs[0].Error()), ""+
					"dependency not resolved: bool\n\n"+
					"Traceback:\n"+
					"  Factory for int")

				equal(t, errors.Is(errs[1], ErrDependencyNotResolved), true)
				equal(t, normalizeSourceLines(errs[1].Error()), ""+
					"dependency not resolved: string\n\n"+
					"Traceback:\n"+
					"  Factory for int32")
			},
		},
		{
			name: "ServiceDuplicatedError",
			options: []Option{
				NewFactory(func() (string, error) { return "s1", nil }),
				NewFactory(func() (string, error) { return "s2", nil }),
				NewEntrypoint(func(string) {}),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrFactoryTypeDuplicated), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 2)

				equal(t, errors.Is(errs[0], ErrFactoryTypeDuplicated), true)
				equal(t, normalizeSourceLines(errs[0].Error()), ""+
					"factory type duplicated: string\n\n"+
					"Traceback:\n"+
					"  Factory for string")

				equal(t, errors.Is(errs[1], ErrFactoryTypeDuplicated), true)
				equal(t, normalizeSourceLines(errs[1].Error()), ""+
					"factory type duplicated: string\n\n"+
					"Traceback:\n"+
					"  Factory for string")
			},
		},
		{
			name: "CircularDependencyErrors",
			options: []Option{
				NewFactory(func(bool) (int, error) { return 1, nil }),
				NewFactory(func(string) (bool, error) { return true, nil }),
				NewFactory(func(int) (string, error) { return "s", nil }),
				NewEntrypoint(func(int, bool, string) {}),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrCircularDependency), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 3)

				equal(t, errors.Is(errs[0], ErrCircularDependency), true)
				equal(t, normalizeSourceLines(errs[0].Error()), ""+
					"circular dependency\n\n"+
					"Traceback:\n"+
					"  Factory for int")

				equal(t, errors.Is(errs[1], ErrCircularDependency), true)
				equal(t, normalizeSourceLines(errs[1].Error()), ""+
					"circular dependency\n\n"+
					"Traceback:\n"+
					"  Factory for bool")

				equal(t, errors.Is(errs[2], ErrCircularDependency), true)
				equal(t, normalizeSourceLines(errs[2].Error()), ""+
					"circular dependency\n\n"+
					"Traceback:\n"+
					"  Factory for string")
			},
		},
		{
			name: "ComplexErrors",
			options: []Option{
				NewFactory(func(struct{ X int }) string { return "s1" }),  // not resolved, duplicate
				NewFactory(func(bool) (int, error) { return 1, nil }),     // cycle
				NewFactory(func(int) (bool, error) { return true, nil }),  // cycle
				NewFactory(func() string { return "s2" }),                 // duplicate
				NewEntrypoint(func(struct{ X int }) error { return nil }), // not resolved
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrDependencyNotResolved), true)
				equal(t, errors.Is(err, ErrFactoryTypeDuplicated), true)
				equal(t, errors.Is(err, ErrCircularDependency), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 6)

				equal(t, errors.Is(errs[0], ErrDependencyNotResolved), true)
				equal(t, normalizeSourceLines(errs[0].Error()), ""+
					"dependency not resolved: struct { X int }\n\n"+
					"Traceback:\n"+
					"  Factory for string")

				equal(t, errors.Is(errs[1], ErrDependencyNotResolved), true)
				equal(t, normalizeSourceLines(errs[1].Error()), ""+
					"dependency not resolved: struct { X int }\n\n"+
					"Traceback:\n"+
					"  Entrypoint")

				equal(t, errors.Is(errs[2], ErrFactoryTypeDuplicated), true)
				equal(t, normalizeSourceLines(errs[2].Error()), ""+
					"factory type duplicated: string\n\n"+
					"Traceback:\n"+
					"  Factory for string")

				equal(t, errors.Is(errs[3], ErrFactoryTypeDuplicated), true)
				equal(t, normalizeSourceLines(errs[3].Error()), ""+
					"factory type duplicated: string\n\n"+
					"Traceback:\n"+
					"  Factory for string")

				equal(t, errors.Is(errs[4], ErrCircularDependency), true)
				equal(t, normalizeSourceLines(errs[4].Error()), ""+
					"circular dependency\n\n"+
					"Traceback:\n"+
					"  Factory for int")

				equal(t, errors.Is(errs[5], ErrCircularDependency), true)
				equal(t, normalizeSourceLines(errs[5].Error()), ""+
					"circular dependency\n\n"+
					"Traceback:\n"+
					"  Factory for bool")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &registry{}
			for _, option := range tt.options {
				equal(t, option.apply(registry), nil)
			}
			tt.wantErr(t, registry.validateRegistry())
		})
	}
}

// TestRegistryInvokeFunctions tests corresponding registry method.
func TestRegistryInvokeFunctions(t *testing.T) {
	registry := &registry{}
	invoked := atomic.Bool{}

	equal(t, NewFactory(func() bool { return true }).apply(registry), nil)
	equal(t, NewEntrypoint(func(_ bool) { invoked.Store(true) }).apply(registry), nil)

	factory := registry.factories[0]
	equal(t, registry.invokeEntrypoints(), nil)
	equal(t, factory.isSpawned, true)
	equal(t, len(factory.outValues), 1)
	equal(t, factory.outValues[0].Interface(), true)
	equal(t, factory.getOutValue().Interface(), true)
	equal(t, invoked.Load(), true)
}

// TestRegistryResolveParallel tests corresponding registry method.
// This test must be run with the race detector (`-race` flag).
func TestRegistryResolveParallel(t *testing.T) {
	invocations := atomic.Int32{}
	source := NewFactory(func() bool {
		invocations.Add(1)
		time.Sleep(10 * time.Millisecond)
		return true
	})

	registry := &registry{}

	equal(t, source.apply(registry), nil)
	factory := registry.factories[0]

	wg := sync.WaitGroup{}
	wg.Add(10)
	for x := 0; x < 10; x++ {
		go func() {
			values, err := registry.resolveByType(reflect.TypeOf(true))
			equal(t, err, nil)
			equal(t, values[0].Interface(), true)
			wg.Done()
		}()
	}

	wg.Wait()
	equal(t, factory.getIsSpawned(), true)
	equal(t, invocations.Load(), int32(1))
}

// TestRegistryResolveFuncServices tests resolving of func services.
func TestRegistryResolveFuncServices(t *testing.T) {
	func1Calls := atomic.Int64{}
	func2Calls := atomic.Int64{}
	fact3Calls := atomic.Int64{}

	options := []Option{
		NewFactory(func() func() int {
			return func() int {
				func1Calls.Add(1)
				return 42
			}
		}),
		NewFactory(func() func(string) string {
			return func(str string) string {
				func2Calls.Add(1)
				return str + " test"
			}
		}),
		NewEntrypoint(func(
			fn1 func() int,
			fn2 func(string) string,
			fn3 func(string) string,
		) {
			fact3Calls.Add(1)
			equal(t, fn1(), 42)
			equal(t, fn2("hello"), "hello test")
			equal(t, fn3("world"), "world test")
			equal(t, fn3("universe"), "universe test")
		}),
	}

	registry := &registry{}
	for _, option := range options {
		equal(t, option.apply(registry), nil)
	}

	err := registry.invokeEntrypoints()
	equal(t, err, nil)

	err = registry.closeFactories()
	equal(t, err, nil)

	equal(t, func1Calls.Load(), int64(1))
	equal(t, func2Calls.Load(), int64(3))
	equal(t, fact3Calls.Load(), int64(1))
}

// TestRegistryResolveWithErrors tests corresponding registry method.
func TestRegistryResolveWithErrors(t *testing.T) {
	source := NewFactory(func() (bool, error) {
		return true, errors.New("some function-specific error message")
	})

	registry := &registry{}
	equal(t, source.apply(registry), nil)

	value, err := registry.resolveService(reflect.TypeOf(true))
	equal(t, err != nil, true)
	equal(t, value.IsValid(), false)
	equal(t, normalizeSourceLines(fmt.Sprint(err)), ""+
		"some function-specific error message\n\n"+
		"Traceback:\n"+
		"  Factory for bool")
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
}

// TestRegistryInvokeWithErrors tests corresponding registry method.
func TestRegistryInvokeWithErrors(t *testing.T) {
	source := NewEntrypoint(func() error {
		return errors.New("some function-specific error message")
	})

	registry := &registry{}
	equal(t, source.apply(registry), nil)

	err := registry.invokeEntrypoints()
	equal(t, err != nil, true)
	equal(t, normalizeSourceLines(fmt.Sprint(err)), ""+
		"some function-specific error message\n\n"+
		"Traceback:\n"+
		"  Entrypoint")
	equal(t, errors.Is(err, ErrEntrypointReturnedError), true)
}

// TestRegistryInvokeEntrypointWithFailedDependency tests that a failed dependency
// factory does not cause a nil-slice panic when the entrypoint returns an error.
func TestRegistryInvokeEntrypointWithFailedDependency(t *testing.T) {
	options := []Option{
		NewFactory(func() (string, error) {
			return "", errors.New("dependency factory error")
		}),
		NewEntrypoint(func(string) error { return nil }),
	}

	registry := &registry{}
	for _, option := range options {
		equal(t, option.apply(registry), nil)
	}

	err := registry.invokeEntrypoints()
	equal(t, err != nil, true)
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
	equal(t, normalizeSourceLines(fmt.Sprint(err)), ""+
		"dependency factory error\n\n"+
		"Traceback:\n"+
		"  Factory for string\n"+
		"  Entrypoint")
}

// TestRegistryInvokeMultipleEntrypointsWithFailedDependency tests that a failed
// dependency on one entrypoint does not abort invocation of the rest.
func TestRegistryInvokeMultipleEntrypointsWithFailedDependency(t *testing.T) {
	secondInvoked := false

	options := []Option{
		NewFactory(func() (string, error) {
			return "", errors.New("dependency factory error")
		}),
		NewEntrypoint(func(string) error { return nil }),
		NewEntrypoint(func() error {
			secondInvoked = true
			return errors.New("second entrypoint error")
		}),
	}

	registry := &registry{}
	for _, option := range options {
		equal(t, option.apply(registry), nil)
	}

	err := registry.invokeEntrypoints()
	equal(t, err != nil, true)
	equal(t, secondInvoked, true)
	equal(t, errors.Is(err, ErrFactoryReturnedError), true)
	equal(t, errors.Is(err, ErrEntrypointReturnedError), true)

	unwrap, ok := err.(interface{ Unwrap() []error })
	equal(t, ok, true)
	errs := unwrap.Unwrap()
	equal(t, len(errs), 2)
	equal(t, normalizeSourceLines(errs[0].Error()), ""+
		"dependency factory error\n\n"+
		"Traceback:\n"+
		"  Factory for string\n"+
		"  Entrypoint")
	equal(t, normalizeSourceLines(errs[1].Error()), ""+
		"second entrypoint error\n\n"+
		"Traceback:\n"+
		"  Entrypoint")
}

// TestIsEmptyInterface tests checking of argument to be empty interface.
func TestIsEmptyInterface(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 interface{ Close() error }

	equal(t, isEmptyInterface(reflect.TypeOf(&t1).Elem()), true)
	equal(t, isEmptyInterface(reflect.TypeOf(&t2).Elem()), true)
	equal(t, isEmptyInterface(reflect.TypeOf(&t3).Elem()), false)
	equal(t, isEmptyInterface(reflect.TypeOf(&t4).Elem()), false)
	equal(t, isEmptyInterface(reflect.TypeOf(&t5).Elem()), false)
}

// TestIsErrorInterface tests checking of argument to be error.
func TestIsErrorInterface(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 error

	equal(t, isErrorInterface(reflect.TypeOf(&t1).Elem()), false)
	equal(t, isErrorInterface(reflect.TypeOf(&t2).Elem()), false)
	equal(t, isErrorInterface(reflect.TypeOf(&t3).Elem()), false)
	equal(t, isErrorInterface(reflect.TypeOf(&t4).Elem()), false)
	equal(t, isErrorInterface(reflect.TypeOf(&t5).Elem()), true)
}
