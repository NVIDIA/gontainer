package gontainer

import (
	"context"
	"reflect"
	"testing"
)

// TestIsOptionalType tests checking of argument to be optional.
func TestIsOptionalType(t *testing.T) {
	var t1 any
	var t2 interface{}
	var t3 struct{}
	var t4 string
	var t5 context.Context
	var t6 Optional[context.Context]

	typ := reflect.TypeOf(&t1).Elem()
	rtyp, ok := isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t2).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t3).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t4).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t5).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, nil)
	equal(t, ok, false)

	typ = reflect.TypeOf(&t6).Elem()
	rtyp, ok = isOptionalType(typ)
	equal(t, rtyp, reflect.TypeOf(&t5).Elem())
	equal(t, ok, true)
}

// TestNewOptionalValue tests creation of optional value.
func TestNewOptionalValue(t *testing.T) {
	// When optional not found.
	box := Optional[string]{}
	data := reflect.New(reflect.TypeOf((*string)(nil)).Elem()).Elem()
	value := newOptionalValue(reflect.TypeOf(box), data)
	equal(t, value.Interface().(Optional[string]).Get(), "")

	// When optional found.
	box = Optional[string]{}
	data = reflect.ValueOf("result")
	value = newOptionalValue(reflect.TypeOf(box), data)
	equal(t, value.Interface().(Optional[string]).Get(), "result")
}
