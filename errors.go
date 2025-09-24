package gontainer

import (
	"errors"
)

// ErrServiceDuplicated declares service duplicated error.
var ErrServiceDuplicated = errors.New("service duplicated")

// ErrServiceNotResolved declares service not resolved error.
var ErrServiceNotResolved = errors.New("service not resolved")

// ErrCircularDependency declares a cyclic dependency error.
var ErrCircularDependency = errors.New("circular dependency")
