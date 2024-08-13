package gontainer

import (
	"reflect"
	"testing"
)

// TestIsOptionalType tests checking of argument to be optional.
func TestIsOptionalType(t *testing.T) {
	var t1 any
	var t2 string
	var t3 Optional[int]

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
	equal(t, rtyp, reflect.TypeOf((*int)(nil)).Elem())
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
