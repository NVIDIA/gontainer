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

// Invoker defines an interface for invoking functions with automatic dependency resolution.
//
// The Invoke method accepts a function `fn`, resolves its input parameters using the invoker’s
// dependency resolver, and then calls the function with the resolved arguments.
//
// If the container has not been started yet, dependency resolution happens in lazy mode — only
// the required arguments and their transitive dependencies are instantiated on demand.
//
// The function may have one of the following return signatures:
//   - no return values (i.e., `func(...)`),
//   - a single `error` (i.e., `func(...) error`),
//   - multiple return values, where the last one may be an `error` (i.e., `func(...) (T1, T2, ..., error)`).
//
// If the last return value implements the `error` interface and is non-nil, it is returned.
// All other return values are collected into the InvokeResult.
//
// An error is also returned if:
//   - `fn` is not a function,
//   - any dependency could not be resolved.
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
