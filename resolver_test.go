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
	"testing"
	"time"
)

// TestResolverService tests resolver service.
func TestResolverService(t *testing.T) {
	container, err := New(
		NewFactory(func() string { return "string" }),
		NewFactory(func(resolver Resolver) {
			var depExists string
			equal(t, resolver.Resolve(&depExists), nil)
			equal(t, depExists, "string")

			var depNotExists int
			equal(t, resolver.Resolve(&depNotExists) != nil, true)
			equal(t, depNotExists, 0)
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Start all factories in the container.
	equal(t, container.Start(), nil)

	// Let async service function launch.
	time.Sleep(time.Millisecond)

	// Close all factories in the container.
	equal(t, container.Close(), nil)

	<-container.Done()
}
