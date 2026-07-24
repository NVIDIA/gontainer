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
	"sync/atomic"
	"testing"
)

// TestResolverService tests resolver service resolve.
func TestResolverService(t *testing.T) {
	svc1 := float64(100500)
	svc2 := float32(100501)

	// Prepare started flag.
	started := atomic.Bool{}

	// Run container.
	equal(t, Run(
		NewService(svc1),
		NewService(svc2),
		NewEntrypoint(func(resolver *Resolver) {
			started.Store(true)

			var depExists float64
			equal(t, resolver.Resolve(&depExists), nil)
			equal(t, depExists, svc1)

			var depNotExists int
			equal(t, resolver.Resolve(&depNotExists) != nil, true)
			equal(t, depNotExists, 0)
		}),
	), nil)

	// Assert started flag is set.
	equal(t, started.Load(), true)
}

// TestResolverResolvePanics verifies that Resolver.Resolve rejects targets that
// are not valid non-nil writable pointers with a stable, gontainer-prefixed
// panic instead of returning an error or leaking a reflect panic.
func TestResolverResolvePanics(t *testing.T) {
	resolver := &Resolver{registry: &registry{}}

	// A typed nil pointer exercises the nil pointer rejection path.
	var nilIntPtr *int

	tests := []struct {
		name string
		call func()
		want string
	}{
		{
			name: "UntypedNil",
			call: func() { _ = resolver.Resolve(nil) },
			want: "expected a non-nil pointer, got nil",
		},
		{
			name: "NonPointerValue",
			call: func() { _ = resolver.Resolve(42) },
			want: "expected a non-nil pointer, got int",
		},
		{
			name: "TypedNilPointer",
			call: func() { _ = resolver.Resolve(nilIntPtr) },
			want: "expected a non-nil pointer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPanics(t, tt.want, tt.call)
		})
	}
}

// TestResolverResolveValidReturnsError verifies that a valid pointer never
// panics: an unregistered type is surfaced as an error for well-formed input.
func TestResolverResolveValidReturnsError(t *testing.T) {
	resolver := &Resolver{registry: &registry{}}

	// A valid pointer to an unregistered type must return an error.
	var dep *testService1
	err := resolver.Resolve(&dep)
	equal(t, err != nil, true)
	equal(t, dep, (*testService1)(nil))
}
