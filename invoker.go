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
	registry *registry
}

// Invoke invokes specified function.
func (i *Invoker) Invoke(function any) ([]any, error) {
	// Get reflection of the function.
	funcValue := reflect.ValueOf(function)
	funcType := reflect.TypeOf(function)
	if funcType == nil || funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid type: %v", funcType)
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
