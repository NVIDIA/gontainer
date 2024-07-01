package gontainer

import (
	"fmt"
	"reflect"
)

// Invoker defines invoker interface.
type Invoker interface {
	// Invoke invokes specified function.
	Invoke(fn any) (InvokeResult, error)
}

// invoker implements invoker interface.
type invoker struct {
	resolver Resolver
}

// Invoke invokes specified function.
func (i *invoker) Invoke(fn any) (InvokeResult, error) {
	// Get reflection of the fn.
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn must be a function")
	}

	// Resolve function arguments.
	fnInArgs := make([]reflect.Value, 0, fnValue.Type().NumIn())
	for index := 0; index < fnValue.Type().NumIn(); index++ {
		fnArgPtrValue := reflect.New(fnValue.Type().In(index))
		if err := i.resolver.Resolve(fnArgPtrValue.Interface()); err != nil {
			return nil, fmt.Errorf("failed to resolve dependency: %w", err)
		}
		fnInArgs = append(fnInArgs, fnArgPtrValue.Elem())
	}

	// Convert function results.
	fnOutArgs := fnValue.Call(fnInArgs)
	result := &invokeResult{
		values: make([]any, 0, len(fnOutArgs)),
		err:    nil,
	}
	for index, fnOut := range fnOutArgs {
		// If it is the last return value.
		if index == len(fnOutArgs)-1 {
			// And type of the value is the error.
			if fnOut.Type().Implements(errorType) {
				// Use the value as an error.
				// Ignore failed cast of nil error.
				result.err, _ = fnOut.Interface().(error)
			}
		}

		// Add value to the results slice.
		result.values = append(result.values, fnOut.Interface())
	}

	return result, nil
}

// InvokeResult provides access to the invocation result.
type InvokeResult interface {
	// Values returns a slice of function result values.
	Values() []any

	// Error returns function result error, if any.
	Error() error
}

// invokeResult implements corresponding interface.
type invokeResult struct {
	values []any
	err    error
}

// Values implements corresponding interface method.
func (r *invokeResult) Values() []any {
	return r.values
}

// Error implements corresponding interface method.
func (r *invokeResult) Error() error {
	return r.err
}
