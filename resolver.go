package gontainer

import (
	"context"
	"fmt"
	"reflect"
)

// Resolver defines service resolver interface.
type Resolver interface {
	// Resolve resolves specified dependency.
	Resolve(varPtr any) error

	// Implements fills a slice with services.
	Implements(slicePtr any) error
}

// resolver implements resolver interface.
type resolver struct {
	ctx      context.Context
	registry *registry
}

// Resolve resolves specified dependency.
func (r *resolver) Resolve(varPtr any) error {
	value := reflect.ValueOf(varPtr).Elem()
	result, err := r.registry.resolveService(value.Type())
	if err != nil {
		return fmt.Errorf("failed to resolve service: %w", err)
	}
	value.Set(result)
	return nil
}

// Implements fills a slice with services.
func (r *resolver) Implements(slicePtr any) error {
	// Validate slice ptr is a pointer.
	slicePtrValue := reflect.ValueOf(slicePtr)
	if slicePtrValue.Kind() != reflect.Ptr {
		return fmt.Errorf("invalid argument: %T is not a pointer", slicePtr)
	}

	// Validate slice ptr points to a slice.
	sliceValue := slicePtrValue.Elem()
	if sliceValue.Kind() != reflect.Slice {
		return fmt.Errorf("invalid argument: %T is not a slice", slicePtrValue)
	}

	// Validate slice elems type is an interface.
	sliceValueType := sliceValue.Type().Elem()
	if sliceValueType.Kind() != reflect.Interface {
		return fmt.Errorf("invalid argument: %T is not an interface", sliceValueType)
	}

	// Get factories for services which implements an interface.
	factories, outputs := r.registry.findFactoriesFor(sliceValueType)
	for index, factory := range factories {
		serviceType := factory.factoryOutTypes[outputs[index]]
		result, err := r.registry.resolveService(serviceType)
		if err != nil {
			return fmt.Errorf("failed to resolve service: %w", err)
		}
		sliceValue = reflect.Append(sliceValue, result)
	}

	// Set slice value on slice pointer.
	slicePtrValue.Elem().Set(sliceValue)
	return nil
}
