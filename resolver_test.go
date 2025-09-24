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
	"sync/atomic"
	"testing"
)

// TestResolverService tests resolver service resolve.
func TestResolverService(t *testing.T) {
	factoryCalled := atomic.Bool{}
	functionCalled := atomic.Bool{}

	svc1 := float64(100500)
	svc2 := float32(100501)

	err := Run(
		context.Background(),
		NewService(svc1),
		NewService(svc2),
		NewFactory(func(resolver *Resolver) bool {
			factoryCalled.Store(true)

			var depExists float64
			equal(t, resolver.Resolve(&depExists), nil)
			equal(t, depExists, svc1)

			return true
		}),
		NewFunction(func(resolver *Resolver) error {
			functionCalled.Store(true)

			var depExists bool
			equal(t, resolver.Resolve(&depExists) == nil, true)
			equal(t, depExists, true)

			var depNotExists int
			equal(t, resolver.Resolve(&depNotExists) != nil, true)
			equal(t, depNotExists, 0)

			return nil
		}),
	)
	equal(t, err, nil)
	equal(t, factoryCalled.Load(), true)
	equal(t, functionCalled.Load(), true)
}
