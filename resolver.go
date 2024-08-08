package gontainer

import (
	"context"
	"fmt"
	"reflect"
)

// Resolver defines service resolver interface.
type Resolver interface {
	// Resolve returns specified dependency.
	Resolve(any) error
}

// resolver implements resolver interface.
type resolver struct {
	ctx      context.Context
	registry *registry
}

// Resolve returns specified dependency.
func (r *resolver) Resolve(varPtr any) error {
	value := reflect.ValueOf(varPtr).Elem()
	result, err := r.registry.resolveService(value.Type())
	if err != nil {
		return fmt.Errorf("failed to get service: %s", err)
	}
	value.Set(result)
	return nil
}
