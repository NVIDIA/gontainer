package gontainer

import (
	"reflect"
)

// Multiple defines multiple service dependencies.
type Multiple[T any] []T

// Multiple marks this type as multiple.
func (m Multiple[T]) Multiple() {}

// isMultipleType checks and returns optional box type.
func isMultipleType(typ reflect.Type) (reflect.Type, bool) {
	if typ.Kind() == reflect.Slice {
		if _, ok := typ.MethodByName("Multiple"); ok {
			return typ.Elem(), true
		}
	}
	return nil, false
}

// newOptionalValue boxes an optional factory input to structs.
func newMultipleValue(typ reflect.Type, values []reflect.Value) reflect.Value {
	box := reflect.New(typ).Elem()
	return reflect.Append(box, values...)
}
