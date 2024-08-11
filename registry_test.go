package gontainer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// TestRegistryRegisterFactory tests corresponding registry method.
func TestRegistryRegisterFactory(t *testing.T) {
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	ctx := context.Background()
	opts := WithMetadata("test", func() {})
	factory := NewFactory(fun, opts)

	registry := &registry{}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.factories, []*Factory{factory})
	equal(t, factory.factoryFunc == nil, false)
	equal(t, factory.factoryLoaded, true)
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
				equal(t, errs[0].Error(), "failed to validate service 'bool' (argument 0) "+
					"of 'Factory[func(bool) error]' from 'github.com/NVIDIA/gontainer': "+
					"service not resolved")

				equal(t, errors.Is(errs[1], ErrServiceNotResolved), true)
				equal(t, errs[1].Error(), "failed to validate service 'string' (argument 0) "+
					"of 'Factory[func(string) error]' from 'github.com/NVIDIA/gontainer': "+
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
				equal(t, errs[0].Error(), "failed to validate service 'string' (output 0) of "+
					"'Factory[func() (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[1], ErrServiceDuplicated), true)
				equal(t, errs[1].Error(), "failed to validate service 'string' (output 0) of "+
					"'Factory[func() (string, error)]' from 'github.com/NVIDIA/gontainer': "+
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
				equal(t, errs[0].Error(), "failed to validate service 'bool' (argument 0) "+
					"of 'Factory[func(bool) (int, error)]' from 'github.com/NVIDIA/gontainer': "+
					"circular dependency")

				equal(t, errors.Is(errs[1], ErrCircularDependency), true)
				equal(t, errs[1].Error(), "failed to validate service 'string' (argument 0) "+
					"of 'Factory[func(string) (bool, error)]' from 'github.com/NVIDIA/gontainer': "+
					"circular dependency")

				equal(t, errors.Is(errs[2], ErrCircularDependency), true)
				equal(t, errs[2].Error(), "failed to validate service 'int' (argument 0) "+
					"of 'Factory[func(int) (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"circular dependency")
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
				equal(t, errs[0].Error(), "failed to validate service 'struct { X int }' (argument 0) "+
					"of 'Factory[func(struct { X int }) string]' from 'github.com/NVIDIA/gontainer': "+
					"service not resolved")

				equal(t, errors.Is(errs[1], ErrServiceDuplicated), true)
				equal(t, errs[1].Error(), "failed to validate service 'string' (output 0) "+
					"of 'Factory[func(struct { X int }) string]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[2], ErrServiceDuplicated), true)
				equal(t, errs[2].Error(), "failed to validate service 'string' (output 0) "+
					"of 'Factory[func(context.Context) (string, error)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")

				equal(t, errors.Is(errs[3], ErrCircularDependency), true)
				equal(t, errs[3].Error(), "failed to validate service 'bool' (argument 0) "+
					"of 'Factory[func(bool) (int, error)]' from 'github.com/NVIDIA/gontainer': "+
					"circular dependency")

				equal(t, errors.Is(errs[4], ErrCircularDependency), true)
				equal(t, errs[4].Error(), "failed to validate service 'int' (argument 0) "+
					"of 'Factory[func(int) (bool, string)]' from 'github.com/NVIDIA/gontainer': "+
					"circular dependency")

				equal(t, errors.Is(errs[5], ErrServiceDuplicated), true)
				equal(t, errs[5].Error(), "failed to validate service 'string' (output 1) "+
					"of 'Factory[func(int) (bool, string)]' from 'github.com/NVIDIA/gontainer': "+
					"service duplicated")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			registry := &registry{}
			for _, factory := range tt.factories {
				equal(t, registry.registerFactory(ctx, factory), nil)
			}
			tt.wantErr(t, registry.validateFactories())
		})
	}
}

// TestRegistryProduceServices tests corresponding registry method.
func TestRegistryProduceServices(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory(func() bool { return true })

	registry := &registry{}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.produceServices(), nil)
	equal(t, factory.factorySpawned, true)

	result := factory.factoryOutValues[0]
	equal(t, result.Interface(), true)
}

// TestRegistryProduceWithErrors tests corresponding registry method.
func TestRegistryProduceWithErrors(t *testing.T) {
	registry := &registry{}
	equal(t, registry.registerFactory(context.Background(), NewFactory(func() (bool, error) {
		return false, errors.New("failed to create new service")
	})), nil)

	err := registry.produceServices()
	equal(t, err != nil, true)
	equal(t, fmt.Sprint(err), `failed to spawn services of `+
		`Factory[func() (bool, error)] from 'github.com/NVIDIA/gontainer': `+
		`factory returned error: failed to create new service`)
}

// TestRegistryCloseServices tests corresponding registry method.
func TestRegistryCloseServices(t *testing.T) {
	funcStarted := atomic.Bool{}
	funcClosed := atomic.Bool{}
	factory := NewFactory(func(ctx context.Context) any {
		return func() error {
			funcStarted.Store(true)
			<-ctx.Done()
			funcClosed.Store(true)
			return nil
		}
	})

	ctx := context.Background()
	registry := &registry{}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.produceServices(), nil)
	equal(t, factory.factorySpawned, true)

	// Let factory function start executing in the background.
	time.Sleep(time.Millisecond)

	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), false)
	equal(t, registry.closeServices(), nil)
	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), true)
}

// TestRegistryCloseWithError tests corresponding registry method.
func TestRegistryCloseWithError(t *testing.T) {
	ctx := context.Background()
	registry := &registry{}

	equal(t, registry.registerFactory(ctx, NewFactory(func(ctx context.Context) any {
		return func() error { return errors.New("failed to close 1") }
	})), nil)

	equal(t, registry.registerFactory(ctx, NewFactory(func() any {
		return func() error { return errors.New("failed to close 2") }
	})), nil)

	equal(t, registry.produceServices(), nil)
	err := registry.closeServices()
	equal(t, err != nil, true)
	equal(t, fmt.Sprint(err), `failed to close services: `+
		`Factory[func() interface {}] from 'github.com/NVIDIA/gontainer': failed to close 2`+"\n"+
		`Factory[func(context.Context) interface {}] from 'github.com/NVIDIA/gontainer': failed to close 1`)
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

// TestCheckIsOptionalIn tests checking of argument to be optional.
func TestCheckIsOptionalIn(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 context.Context
	var t6 Optional[context.Context]

	typ := reflect.TypeOf(&t1).Elem()
	rtyp, ok := isOptionalBoxType(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t2).Elem()
	rtyp, ok = isOptionalBoxType(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t3).Elem()
	rtyp, ok = isOptionalBoxType(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t4).Elem()
	rtyp, ok = isOptionalBoxType(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t5).Elem()
	rtyp, ok = isOptionalBoxType(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t6).Elem()
	rtyp, ok = isOptionalBoxType(typ)
	equal(t, rtyp, reflect.TypeOf(&t5).Elem())
	equal(t, ok, true)
}

// TestBoxFactoryOptionalIn tests boxing of factory output to optional box.
func TestBoxFactoryOptionalIn(t *testing.T) {
	// When optional not found.
	box := Optional[string]{}
	data := reflect.New(reflect.TypeOf((*string)(nil)).Elem()).Elem()
	value := getOptionalBox(reflect.TypeOf(box), data)
	equal(t, value.Interface().(Optional[string]).Get(), "")

	// When optional found.
	box = Optional[string]{}
	data = reflect.ValueOf("result")
	value = getOptionalBox(reflect.TypeOf(box), data)
	equal(t, value.Interface().(Optional[string]).Get(), "result")
}

// TestBoxFactoryOutFunc tests conversion of service function to closable service object.
func TestBoxFactoryOutFunc(t *testing.T) {
	var result = errors.New("test")
	var svcfunc any = func() error {
		return result
	}

	svcvalue := reflect.ValueOf(&svcfunc).Elem()
	wrapper, err := wrapFactoryFunc(svcvalue)
	equal(t, err, nil)

	service := wrapper.Interface().(function)
	equal(t, service.Close(), result)
}
