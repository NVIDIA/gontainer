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

// TestRegistryStartFactories tests corresponding registry method.
func TestRegistryStartFactories(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory(func() bool { return true })

	registry := &registry{}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.produceServices(), nil)
	equal(t, factory.factorySpawned, true)

	result := factory.factoryOutValues[0]
	equal(t, result.Interface(), true)
}

// TestRegistryStartWithErrors tests corresponding registry method.
func TestRegistryStartWithErrors(t *testing.T) {
	registry := &registry{}
	equal(t, registry.registerFactory(context.Background(), NewFactory(func() (bool, error) {
		return false, errors.New("failed to create new service")
	})), nil)

	err := registry.produceServices()
	equal(t, err != nil, true)
	equal(t, fmt.Sprint(err), `failed to spawn services of `+
		`Factory[func() (bool, error)] from 'github.com/NVIDIA/gontainer': `+
		`failed to invoke factory: failed to create new service`)
}

// TestRegistryCloseFactories tests corresponding registry method.
func TestRegistryCloseFactories(t *testing.T) {
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
