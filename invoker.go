package gontainer

import (
	"context"
	"fmt"
	"reflect"
)

// Invoker defines function invoker interface.
type Invoker interface {
	// Invoke calls a function and returns result.
	Invoke(fn any) ([]any, error)
}

// invoker implements invoker interface.
type invoker struct {
	ctx      context.Context
	registry *registry
}

// Invoke calls a function and returns its result.
func (i *invoker) Invoke(fn any) ([]any, error) {
	// Get function reflection value.
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		panic(fmt.Sprintf("invalid function type: %s", fnValue.Type()))
	}

	// Prepare function arguments types slice.
	inTypes := make([]reflect.Type, 0, fnValue.Type().NumIn())
	for index := 0; index < fnValue.Type().NumIn(); index++ {
		inTypes = append(inTypes, fnValue.Type().In(index))
	}

	// Prepare function arguments values slice.
	inValues, err := i.registry.getFactoryIns(i.ctx, inTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to get arguments: %w", err)
	}

	// Call function and get out values slice.
	outValues := fnValue.Call(inValues)
	outValuesAny := make([]any, 0, fnValue.Type().NumOut())
	for index := 0; index < fnValue.Type().NumOut(); index++ {
		outValuesAny = append(outValuesAny, outValues[index].Interface())
	}

	return outValuesAny, nil
}
