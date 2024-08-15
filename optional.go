package gontainer

import (
	"reflect"
	"unsafe"
)

// Optional defines optional service dependency.
type Optional[T any] struct {
	value T
}

// Get returns optional service instance.
func (o Optional[T]) Get() T {
	return o.value
}

// Optional marks this type as optional.
func (o Optional[T]) Optional() {}

// isOptionalType checks and returns optional box type.
func isOptionalType(typ reflect.Type) (reflect.Type, bool) {
	if typ.Kind() == reflect.Struct {
		if _, ok := typ.MethodByName("Optional"); ok {
			if methodValue, ok := typ.MethodByName("Get"); ok {
				if methodValue.Type.NumOut() == 1 {
					methodType := methodValue.Type.Out(0)
					return methodType, true
				}
			}
		}
	}
	return nil, false
}

// newOptionalValue creates new optional type with a value.
func newOptionalValue(typ reflect.Type, value reflect.Value) reflect.Value {
	// Prepare boxing struct for value.
	box := reflect.New(typ).Elem()

	// Inject factory output value to the boxing struct.
	field := box.FieldByName("value")
	pointer := unsafe.Pointer(field.UnsafeAddr())
	public := reflect.NewAt(field.Type(), pointer)
	public.Elem().Set(value)

	return box
}
