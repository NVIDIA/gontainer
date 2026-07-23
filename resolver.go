/*
 * SPDX-FileCopyrightText: Copyright (c) 2003 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gontainer

import (
	"fmt"
	"reflect"
)

// Resolver resolves service dependencies.
//
// The Resolve method accepts a non-nil pointer to a variable and populates it with an instance
// of the requested type. The type is determined via reflection from the element the pointer
// refers to.
//
// If the container has not been started yet, Resolve operates in lazy mode — it instantiates
// only the requested type and its transitive dependencies on demand.
//
// Resolve panics when its target argument is not a valid, non-nil, writable pointer; see the
// Resolve method for details. For a valid pointer, an error is returned if the service of the
// requested type is not found or cannot be resolved.
type Resolver struct {
	registry *registry
}

// Resolve populates the target variable with the resolved service.
//
// Resolve validates its target argument at the public API boundary and panics on
// a programmer error: when target is an untyped nil, is not a pointer, is a nil
// pointer, or points to a value that cannot be set. The panic message is prefixed
// with "gontainer:". For a valid pointer, Resolve returns an error when the
// requested service is not found or cannot be resolved.
func (r *Resolver) Resolve(target any) error {
	// Validate the target is not a nil.
	pointerType := reflect.TypeOf(target)
	if pointerType == nil {
		panic(fmt.Sprintf("%s Resolver.Resolve: expected a non-nil pointer, got nil", panicPrefix))
	}

	// Validate the target type is a pointer.
	if pointerType.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("%s Resolver.Resolve: expected a non-nil pointer, got %s", panicPrefix, pointerType))
	}

	// Validate the target value is not a nil pointer.
	pointerValue := reflect.ValueOf(target)
	if pointerValue.IsNil() {
		panic(fmt.Sprintf("%s Resolver.Resolve: expected a non-nil pointer, got nil %s", panicPrefix, pointerType))
	}

	// Validate the target value is a writable value.
	value := pointerValue.Elem()
	if !value.CanSet() {
		panic(fmt.Sprintf("%s Resolver.Resolve: expected a pointer to a writable value, got %s", panicPrefix, pointerType))
	}

	// Resolve the service by the target element type.
	result, err := r.registry.resolveService(value.Type())
	if err != nil {
		return err
	}

	// Populate the target with the resolved service.
	value.Set(result)
	return nil
}
