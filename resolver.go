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

// Resolver defines an interface for resolving service dependencies.
//
// The Resolve method accepts a pointer to a variable (`varPtr`) and attempts to populate it
// with an instance of the requested type. The type is determined via reflection based on the
// element type of `varPtr`.
//
// If the container has not been started yet, Resolve operates in lazy mode — it instantiates
// only the requested type and its transitive dependencies on demand.
//
// An error is returned if the service of the requested type is not found or cannot be resolved.
type Resolver interface {
	// Resolve sets the required dependency via the pointer.
	Resolve(varPtr any) error
}

// resolver implements resolver interface.
type resolver struct {
	registry *registry
}

// Resolve sets the required dependency via the pointer.
func (r *resolver) Resolve(varPtr any) error {
	value := reflect.ValueOf(varPtr).Elem()
	result, err := r.registry.resolveService(value.Type())
	if err != nil {
		return fmt.Errorf("failed to resolve service: %w", err)
	}
	value.Set(result)
	return nil
}
