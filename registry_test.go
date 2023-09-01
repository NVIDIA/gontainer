package gontainer

import (
	"context"
	"reflect"
	"testing"
)

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
