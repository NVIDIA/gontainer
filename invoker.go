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
// The Invoke method accepts a function `fn`, resolves its input parameters using the invoker's
// dependency resolver, and then calls the function with the resolved arguments.
//
// If the container has not been started yet, dependency resolution happens in lazy mode — only
// the required arguments and their transitive dependencies are instantiated on demand.
//
// The Invoke method returns:
//   - []any - all values returned by the function (including any errors)
//   - error - only if dependency resolution fails or fn is not a function
//
// All return values from the invoked function are collected in the []any slice,
// including any error values. The caller is responsible for checking and handling
// these values as appropriate.
type Invoker struct {
	resolver *Resolver
}

// Invoke invokes specified function.
func (i *Invoker) Invoke(fn any) ([]any, error) {
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

	// Call the function and collect results.
	fnOutArgs := fnValue.Call(fnInArgs)
	values := make([]any, 0, len(fnOutArgs))
	for _, fnOut := range fnOutArgs {
		values = append(values, fnOut.Interface())
	}

	return values, nil
}
