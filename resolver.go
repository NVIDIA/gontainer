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
	"context"
	"fmt"
	"reflect"
)

// Resolver defines service resolver interface.
type Resolver interface {
	// Resolve returns specified dependency.
	Resolve(varPtr any) error
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
		return fmt.Errorf("failed to resolve service: %w", err)
	}
	value.Set(result)
	return nil
}
