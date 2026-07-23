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

// Invoker invokes functions with automatic dependency resolution.
//
// The Invoke method accepts a function, resolves its input parameters using the invoker's
// dependency resolver, and then calls the function with the resolved arguments.
//
// If the container has not been started yet, dependency resolution happens in lazy mode — only
// the required arguments and their transitive dependencies are instantiated on demand.
//
// The Invoke method returns:
//   - []any - all values returned by the function (including any errors)
//   - error - only if dependency resolution fails
//
// All return values from the invoked function are collected in the []any slice,
// including any error values. The caller is responsible for checking and handling
// these values as appropriate.
//
// Invoke panics when its function argument is not a valid, non-nil function; see
// the Invoke method for details.
type Invoker struct {
	registry *registry
}

// Invoke invokes the specified function with dependencies resolved from the container.
//
// Invoke validates its function argument at the public API boundary and panics
// on a programmer error: when function is an untyped nil, is not a function, or
// is a typed nil function. The panic message is prefixed with "gontainer:". For
// a valid function, Invoke returns an error only when a dependency cannot be
// resolved; errors produced by the function itself are returned among the []any
// results, not as the error.
func (i *Invoker) Invoke(function any) ([]any, error) {
	// Validate the function is not a nil.
	funcType := reflect.TypeOf(function)
	if funcType == nil {
		panic(fmt.Sprintf("%s Invoker.Invoke: expected a function, got nil", panicPrefix))
	}

	// Validate the function type is a function.
	if funcType.Kind() != reflect.Func {
		panic(fmt.Sprintf("%s Invoker.Invoke: expected a function, got %s", panicPrefix, funcType))
	}

	// Validate the function value is not a typed nil.
	funcValue := reflect.ValueOf(function)
	if funcValue.IsNil() {
		panic(fmt.Sprintf("%s Invoker.Invoke: expected a non-nil function, got nil %s", panicPrefix, funcType))
	}

	// Resolve function arguments.
	inArgs := make([]reflect.Value, 0, funcType.NumIn())
	for index := 0; index < funcType.NumIn(); index++ {
		result, err := i.registry.resolveService(funcType.In(index))
		if err != nil {
			return nil, err
		}
		inArgs = append(inArgs, result)
	}

	// Call the function and collect results.
	outArgs := funcValue.Call(inArgs)
	results := make([]any, 0, len(outArgs))
	for _, fnOut := range outArgs {
		results = append(results, fnOut.Interface())
	}

	return results, nil
}
