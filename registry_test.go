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
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	opts := WithMetadata("test", func() {})
	source := NewFactory(fun, opts)
	factory, err := source.factory()
	equal(t, err, nil)

	registry := &registry{}
	registry.registerFactory(factory)
	equal(t, registry.factories[0], factory)
	equal(t, factory.source.fn == nil, false)
}

// TestRegistryValidateFactories tests corresponding registry method.
func TestRegistryValidateFactories(t *testing.T) {
	tests := []struct {
		name      string
		factories []*Factory
		wantErr   func(t *testing.T, err error)
	}{
		{
			name: "NoValidationErrors",
			factories: []*Factory{
				NewFactory(func(bool) (int, error) { return 1, nil }),
				NewFactory(func(string) (bool, error) { return true, nil }),
				NewFactory(func() (string, error) { return "s", nil }),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, err, nil)
			},
		},
		{
			name: "ServiceNotResolvedError",
			factories: []*Factory{
				NewFactory(func(bool) error { return nil }),
				NewFactory(func(string) error { return nil }),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrServiceNotResolved), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 2)

				equal(t, errors.Is(errs[0], ErrServiceNotResolved), true)
				equal(t, errs[0].Error(), "failed to validate argument 'bool' (index 0) "+
					"of factory 'Factory[func(bool) error]' from 'github.com/NVIDIA/gontainer': "+
					"service not resolved")

				equal(t, errors.Is(errs[1], ErrServiceNotResolved), true)
				equal(t, errs[1].Error(), "failed to validate argument 'string' (index 0) "+
					"of factory 'Factory[func(string) error]' from 'github.com/NVIDIA/gontainer': "+
					"service not resolved")
			},
		},
		{
			name: "ServiceDuplicatedError",
			factories: []*Factory{
				NewFactory(func() (string, error) { return "s1", nil }),
				NewFactory(func() (string, error) { return "s2", nil }),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrServiceDuplicated), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 2)

				equal(t, errors.Is(errs[0], ErrServiceDuplicated), true)
				equal(t, errs[0].Error(), "failed to validate output 'string' (index 0) "+
					"of factory 'Factory[func() (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[1], ErrServiceDuplicated), true)
				equal(t, errs[1].Error(), "failed to validate output 'string' (index 0) "+
					"of factory 'Factory[func() (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")
			},
		},
		{
			name: "CircularDependencyErrors",
			factories: []*Factory{
				NewFactory(func(bool) (int, error) { return 1, nil }),
				NewFactory(func(string) (bool, error) { return true, nil }),
				NewFactory(func(int) (string, error) { return "s", nil }),
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrCircularDependency), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 3)

				equal(t, errors.Is(errs[0], ErrCircularDependency), true)
				equal(t, errs[0].Error(), "failed to validate factory 'Factory[func(bool) (int, error)]' "+
					"from 'github.com/NVIDIA/gontainer': circular dependency")

				equal(t, errors.Is(errs[1], ErrCircularDependency), true)
				equal(t, errs[1].Error(), "failed to validate factory 'Factory[func(string) (bool, error)]' "+
					"from 'github.com/NVIDIA/gontainer': circular dependency")

				equal(t, errors.Is(errs[2], ErrCircularDependency), true)
				equal(t, errs[2].Error(), "failed to validate factory 'Factory[func(int) (string, error)]' "+
					"from 'github.com/NVIDIA/gontainer': circular dependency")
			},
		},
		{
			name: "ComplexErrors",
			factories: []*Factory{
				NewFactory(func(struct{ X int }) string { return "s1" }),                   // not resolved, duplicate
				NewFactory(func(ctx context.Context) (string, error) { return "s2", nil }), // duplicate
				NewFactory(func(bool) (int, error) { return 1, nil }),                      // cycle
				NewFactory(func(int) (bool, string) { return true, "s3" }),                 // cycle, duplicate
			},
			wantErr: func(t *testing.T, err error) {
				equal(t, errors.Is(err, ErrServiceNotResolved), true)
				equal(t, errors.Is(err, ErrServiceDuplicated), true)
				equal(t, errors.Is(err, ErrCircularDependency), true)

				unwrap, ok := err.(interface{ Unwrap() []error })
				equal(t, ok, true)
				errs := unwrap.Unwrap()
				equal(t, len(errs), 6)

				equal(t, errors.Is(errs[0], ErrServiceNotResolved), true)
				equal(t, errs[0].Error(), "failed to validate argument 'struct { X int }' (index 0) "+
					"of factory 'Factory[func(struct { X int }) string]' from 'github.com/NVIDIA/gontainer': "+
					"service not resolved")

				equal(t, errors.Is(errs[1], ErrServiceDuplicated), true)
				equal(t, errs[1].Error(), "failed to validate output 'string' (index 0) "+
					"of factory 'Factory[func(struct { X int }) string]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[2], ErrServiceDuplicated), true)
				equal(t, errs[2].Error(), "failed to validate output 'string' (index 0) "+
					"of factory 'Factory[func(context.Context) (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[3], ErrServiceDuplicated), true)
				equal(t, errs[3].Error(), "failed to validate output 'string' (index 1) "+
					"of factory 'Factory[func(int) (bool, string)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[4], ErrCircularDependency), true)
				equal(t, errs[4].Error(), "failed to validate factory 'Factory[func(bool) (int, error)]' "+
					"from 'github.com/NVIDIA/gontainer': circular dependency")

				equal(t, errors.Is(errs[5], ErrCircularDependency), true)
				equal(t, errs[5].Error(), "failed to validate factory 'Factory[func(int) (bool, string)]' "+
					"from 'github.com/NVIDIA/gontainer': circular dependency")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &registry{}
			for _, source := range tt.factories {
				factory, err := source.factory()
				equal(t, err, nil)
				equal(t, factory == nil, false)
				registry.registerFactory(factory)
			}
			tt.wantErr(t, registry.validateFactories())
		})
	}
}

// TestRegistrySpawnFactories tests corresponding registry method.
func TestRegistrySpawnFactories(t *testing.T) {
	source := NewFactory(func() bool { return true })

	factory, err := source.factory()
	equal(t, err, nil)
	equal(t, factory == nil, false)

	registry := &registry{}
	registry.registerFactory(factory)

	err = registry.spawnFactories()
	equal(t, err, nil)
	equal(t, factory.spawned, true)
	equal(t, factory.outValues[0].Interface(), true)
}

// TestRegistryResolveParallel tests corresponding registry method.
// This test must be run with the race detector (`-race` flag).
func TestRegistryResolveParallel(t *testing.T) {
	source := NewFactory(func() bool {
		time.Sleep(10 * time.Millisecond)
		return true
	})

	factory, err := source.factory()
	equal(t, err, nil)
	equal(t, factory == nil, false)

	registry := &registry{}
	registry.registerFactory(factory)

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
	equal(t, factory.getSpawned(), true)
}

// TestRegistrySpawnWithNil tests resolving of function services.
// Function services are regular services and could be resolved.
func TestRegistryResolveFuncServices(t *testing.T) {
	func1Calls := atomic.Int64{}
	func2Calls := atomic.Int64{}
	fact3Calls := atomic.Int64{}

	sources := []*Factory{
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
		NewFactory(func(
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
	for _, source := range sources {
		factory, err := source.factory()
		equal(t, err, nil)
		equal(t, factory == nil, false)
		registry.registerFactory(factory)
	}

	err := registry.spawnFactories()
	equal(t, err, nil)

	err = registry.closeFactories()
	equal(t, err, nil)

	equal(t, func1Calls.Load(), int64(1))
	equal(t, func2Calls.Load(), int64(3))
	equal(t, fact3Calls.Load(), int64(1))
}

// TestResolveServiceFunc tests resolving of service functions.
// Service functions runs automatically by the container in background.
func TestRegistrySpawnServiceFuncs(t *testing.T) {
	func1Called := atomic.Bool{}
	func2Called := atomic.Bool{}
	func3Called := atomic.Bool{}
	func4Called := atomic.Bool{}

	wg := sync.WaitGroup{}
	wg.Add(4)

	sources := []*Factory{
		NewFactory(func() any {
			return func() {
				func1Called.Store(true)
				wg.Done()
			}
		}),
		NewFactory(func() any {
			return func() error {
				func2Called.Store(true)
				wg.Done()
				return nil
			}
		}),
		NewFactory(func() func() {
			return func() {
				func3Called.Store(true)
				wg.Done()
			}
		}),
		NewFactory(func(ctx context.Context) func() error {
			return func() error {
				func4Called.Store(true)
				<-ctx.Done()
				wg.Done()
				return nil
			}
		}),
	}

	registry := &registry{}
	for _, source := range sources {
		factory, err := source.factory()
		equal(t, err, nil)
		equal(t, factory == nil, false)
		registry.registerFactory(factory)
	}

	err := registry.spawnFactories()
	equal(t, err, nil)

	err = registry.closeFactories()
	equal(t, err, nil)

	wg.Wait()
	equal(t, func1Called.Load(), true)
	equal(t, func2Called.Load(), true)
	equal(t, func3Called.Load(), true)
	equal(t, func4Called.Load(), true)
}

// TestRegistrySpawnWithErrors tests corresponding registry method.
func TestRegistrySpawnWithErrors(t *testing.T) {
	source := NewFactory(func() (bool, error) {
		return false, errors.New("failed to create new service")
	})

	factory, err := source.factory()
	equal(t, err, nil)
	equal(t, factory == nil, false)

	registry := &registry{}
	registry.registerFactory(factory)

	err = registry.spawnFactories()
	equal(t, err != nil, true)
	equal(t, fmt.Sprint(err), `failed to spawn services of `+
		`'Factory[func() (bool, error)]' from 'github.com/NVIDIA/gontainer': `+
		`factory returned error: failed to create new service`)
}

// TestRegistryCloseFactories tests corresponding registry method.
func TestRegistryCloseFactories(t *testing.T) {
	funcStarted := atomic.Bool{}
	funcClosed := atomic.Bool{}
	source := NewFactory(func(ctx context.Context) any {
		return func() error {
			funcStarted.Store(true)
			<-ctx.Done()
			funcClosed.Store(true)
			return nil
		}
	})

	factory, err := source.factory()
	equal(t, err, nil)
	equal(t, factory == nil, false)

	registry := &registry{}
	registry.registerFactory(factory)
	equal(t, registry.spawnFactories(), nil)
	equal(t, factory.spawned, true)

	// Let factory function start executing in the background.
	time.Sleep(time.Millisecond)

	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), false)
	equal(t, registry.closeFactories(), nil)
	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), true)
}

// TestRegistryCloseWithError tests corresponding registry method.
func TestRegistryCloseWithError(t *testing.T) {
	source1 := NewFactory(func(ctx context.Context) any {
		return func() error { return errors.New("failed to close 1") }
	})
	source2 := NewFactory(func() any {
		return func() error { return errors.New("failed to close 2") }
	})

	factory1, err := source1.factory()
	equal(t, err, nil)
	equal(t, factory1 == nil, false)

	factory2, err := source2.factory()
	equal(t, err, nil)
	equal(t, factory2 == nil, false)

	registry := &registry{}
	registry.registerFactory(factory1)
	registry.registerFactory(factory2)

	err = registry.spawnFactories()
	equal(t, err, nil)

	err = registry.closeFactories()
	equal(t, err != nil, true)
	equal(t, fmt.Sprint(err), ``+
		`failed to close service 'gontainer.funcResult' (index 0) of factory `+
		`'Factory[func() interface {}]' from 'github.com/NVIDIA/gontainer': failed to close 2`+"\n"+
		`failed to close service 'gontainer.funcResult' (index 0) of factory `+
		`'Factory[func(context.Context) interface {}]' from 'github.com/NVIDIA/gontainer': failed to close 1`)
}

// TestIsNonEmptyInterface tests checking of argument to be non-empty interface.
func TestIsNonEmptyInterface(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 interface{ Close() error }

	equal(t, isNonEmptyInterface(reflect.TypeOf(&t1).Elem()), false)
	equal(t, isNonEmptyInterface(reflect.TypeOf(&t2).Elem()), false)
	equal(t, isNonEmptyInterface(reflect.TypeOf(&t3).Elem()), false)
	equal(t, isNonEmptyInterface(reflect.TypeOf(&t4).Elem()), false)
	equal(t, isNonEmptyInterface(reflect.TypeOf(&t5).Elem()), true)
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

// TestIsContextInterface tests checking of argument to be context.
func TestIsContextInterface(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 context.Context

	equal(t, isContextInterface(reflect.TypeOf(&t1).Elem()), false)
	equal(t, isContextInterface(reflect.TypeOf(&t2).Elem()), false)
	equal(t, isContextInterface(reflect.TypeOf(&t3).Elem()), false)
	equal(t, isContextInterface(reflect.TypeOf(&t4).Elem()), false)
	equal(t, isContextInterface(reflect.TypeOf(&t5).Elem()), true)
}

// TestIsServiceFunc tests checking of service functions.
func TestIsServiceFunc(t *testing.T) {
	svcErr := errors.New("test")
	svcFunc := func() error { return svcErr }

	tests := []struct {
		name  string
		arg1  func() reflect.Value
		want1 reflect.Value
		want2 bool
	}{{
		name: "AnyTypeVarWithFunc",
		arg1: func() reflect.Value {
			var svcFuncAny any = svcFunc
			return reflect.ValueOf(&svcFuncAny).Elem()
		},
		want1: reflect.ValueOf(svcFunc),
		want2: true,
	}, {
		name: "FuncTypeVarWithFunc",
		arg1: func() reflect.Value {
			var svcFuncTyped = svcFunc
			return reflect.ValueOf(&svcFuncTyped).Elem()
		},
		want1: reflect.ValueOf(&svcFunc).Elem(),
		want2: true,
	}, {
		name: "FuncWithReceivers",
		arg1: func() reflect.Value {
			var svcFuncWrapped funcWithReceivers = svcFunc
			return reflect.ValueOf(&svcFuncWrapped).Elem()
		},
		want1: reflect.Value{},
		want2: false,
	}, {
		name: "AnyTypeVarWithInt",
		arg1: func() reflect.Value {
			var intValue any = 5
			return reflect.ValueOf(&intValue).Elem()
		},
		want1: reflect.Value{},
		want2: false,
	}, {
		name: "IntTypeVarWithInt",
		arg1: func() reflect.Value {
			intValue := 5
			return reflect.ValueOf(&intValue).Elem()
		},
		want1: reflect.Value{},
		want2: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, got2 := isServiceFunc(tt.arg1())
			if (got1.IsValid() || tt.want1.IsValid()) && (got1.Pointer() != tt.want1.Pointer()) {
				t.Errorf("isServiceFunc() got1 = %s, want1 %s", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("isServiceFunc() got2 = %v, want2 %v", got2, tt.want2)
			}
		})
	}
}

// TestWrapServiceFunc tests wrapping of service functions.
func TestWrapServiceFunc(t *testing.T) {
	svcErr := errors.New("test")
	svcFunc1 := func() error { return svcErr }
	svcFunc2 := func() {}

	tests := []struct {
		name  string
		arg1  func() reflect.Value
		want1 error
		want2 error
	}{{
		name: "AnyTypeVarWithFunc1",
		arg1: func() reflect.Value {
			var svcFuncAny any = svcFunc1
			return reflect.ValueOf(&svcFuncAny).Elem()
		},
		want1: svcErr,
		want2: nil,
	}, {
		name: "FuncTypeVarWithFunc1",
		arg1: func() reflect.Value {
			var svcFuncTyped = svcFunc1
			return reflect.ValueOf(&svcFuncTyped).Elem()
		},
		want1: svcErr,
		want2: nil,
	}, {
		name: "FuncTypeVarWithFunc2",
		arg1: func() reflect.Value {
			var svcFuncTyped = svcFunc2
			return reflect.ValueOf(&svcFuncTyped).Elem()
		},
		want1: nil,
		want2: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, got2 := startServiceFunc(tt.arg1())
			if !reflect.DeepEqual(got1.Interface().(funcResult).Close(), tt.want1) {
				t.Errorf("startServiceFunc() got1 = %v, want1 %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("startServiceFunc() got2 = %v, want2 %v", got2, tt.want2)
			}
		})
	}
}

// funcWithReceivers is a function type with receivers.
type funcWithReceivers func() error

// Error defines example method on the service func.
func (f funcWithReceivers) Error() string {
	return "error"
}
