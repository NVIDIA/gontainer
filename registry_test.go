package gontainer

import (
	"context"
	"errors"
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
	opts := WithSubscribe("test", func() {})
	factory := NewFactory(fun, opts)

	registry := &registry{events: events{}}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.factories, []*Factory{factory})
	equal(t, factory.factoryFunc == nil, false)
	equal(t, factory.factoryLoaded, true)
}

// TestRegistryStartFactories tests corresponding registry method.
func TestRegistryStartFactories(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory(func() bool { return true })

	registry := &registry{events: events{}}
	equal(t, registry.registerFactory(ctx, factory), nil)
	equal(t, registry.startFactories(), nil)
	equal(t, factory.factorySpawned, true)

	result := factory.factoryOutValues[0]
	equal(t, result.Interface(), true)
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
	equal(t, registry.startFactories(), nil)
	equal(t, factory.factorySpawned, true)

	// Let factory function start executing in the background.
	time.Sleep(time.Millisecond)

	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), false)
	equal(t, registry.closeFactories(), nil)
	equal(t, funcStarted.Load(), true)
	equal(t, funcClosed.Load(), true)
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
	rtyp, ok := checkIsOptionalIn(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t2).Elem()
	rtyp, ok = checkIsOptionalIn(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t3).Elem()
	rtyp, ok = checkIsOptionalIn(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t4).Elem()
	rtyp, ok = checkIsOptionalIn(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t5).Elem()
	rtyp, ok = checkIsOptionalIn(typ)
	equal(t, rtyp, typ)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t6).Elem()
	rtyp, ok = checkIsOptionalIn(typ)
	equal(t, rtyp, reflect.TypeOf(&t5).Elem())
	equal(t, ok, true)
}

// TestBoxFactoryOptionalIn tests boxing of factory output to optional box.
func TestBoxFactoryOptionalIn(t *testing.T) {
	// Prepare factory description instance.
	result := "result"
	outvals := []reflect.Value{reflect.ValueOf(result)}
	factory := &Factory{factoryOutValues: outvals}

	// When optional misses.
	box := Optional[string]{}
	value := boxFactoryOptionalIn(reflect.TypeOf(box), nil, 0)
	equal(t, value.Interface().(Optional[string]).Get(), "")

	// When optional found.
	box = Optional[string]{}
	value = boxFactoryOptionalIn(reflect.TypeOf(box), factory, 0)
	equal(t, value.Interface().(Optional[string]).Get(), "result")
}

// TestBoxFactoryOutFunc tests conversion of service function to closable service object.
func TestBoxFactoryOutFunc(t *testing.T) {
	var result = errors.New("test")
	var svcfunc any = func() error {
		return result
	}

	svcvalue := reflect.ValueOf(&svcfunc).Elem()
	wrapper, err := boxFactoryOutFunc(svcvalue)
	equal(t, err, nil)

	service := wrapper.Interface().(function)
	equal(t, service.Close(), result)
}
