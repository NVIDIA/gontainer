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
	"errors"
	"sync/atomic"
	"testing"
)

// TestInvokerService tests invoker service.
func TestInvokerService(t *testing.T) {
	tests := []struct {
		name    string
		haveFn  any
		wantFn  func(t *testing.T, values []any)
		wantErr bool
	}{
		{
			name:   "ReturnNothing",
			haveFn: func(var1 string, var2 int) {},
			wantFn: func(t *testing.T, values []any) {
				equal(t, len(values), 0)
			},
			wantErr: false,
		},
		{
			name: "ReturnValuesNoError",
			haveFn: func(var1 string, var2 int) (string, int, bool) {
				return var1 + "-X", var2 + 100, true
			},
			wantFn: func(t *testing.T, values []any) {
				equal(t, len(values), 3)
				equal(t, values[0], "string-X")
				equal(t, values[1], 223)
				equal(t, values[2], true)
			},
			wantErr: false,
		},
		{
			name: "ReturnValuesWithError",
			haveFn: func(var1 string, var2 int) (string, int, error) {
				return var1 + "-X", var2 + 100, errors.New("failed")
			},
			wantFn: func(t *testing.T, values []any) {
				equal(t, len(values), 3)
				equal(t, values[0], "string-X")
				equal(t, values[1], 223)
				// Error is returned as a regular value
				equal(t, values[2].(error).Error(), "failed")
			},
			wantErr: false,
		},
		{
			name: "ReturnMultipleError",
			haveFn: func(var1 string, var2 int) (error, error, error) {
				return nil, errors.New("error-1"), errors.New("error-2")
			},
			wantFn: func(t *testing.T, values []any) {
				equal(t, len(values), 3)
				equal(t, values[0], nil)
				equal(t, values[1].(error).Error(), "error-1")
				equal(t, values[2].(error).Error(), "error-2")
			},
			wantErr: false,
		},
		{
			name:   "ReturnNilError",
			haveFn: func(var1 string, var2 int) error { return nil },
			wantFn: func(t *testing.T, values []any) {
				equal(t, len(values), 1)
				equal(t, values[0], nil)
			},
			wantErr: false,
		},
		{
			// A well-formed function whose dependency is unregistered must
			// surface the failure as an error rather than a panic.
			name:    "UnresolvedDependencyReturnsError",
			haveFn:  func(dep float64) {},
			wantFn:  nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare started flag.
			started := atomic.Bool{}

			// Run container.
			equal(t, Run(
				NewFactory(func() string { return "string" }),
				NewFactory(func() int { return 123 }),
				NewEntrypoint(func(invoker *Invoker) {
					started.Store(true)

					values, err := invoker.Invoke(tt.haveFn)
					if (err != nil) != tt.wantErr {
						t.Errorf("Invoke() error = %v, wantErr %v", err, tt.wantErr)
					}

					if tt.wantFn != nil {
						tt.wantFn(t, values)
					}
				}),
			), nil)

			// Assert started flag is set.
			equal(t, started.Load(), true)
		})
	}
}

// TestInvokerInvokePanics verifies that Invoker.Invoke rejects invalid function
// arguments with a stable, gontainer-prefixed panic instead of returning an
// error or leaking a reflect panic.
func TestInvokerInvokePanics(t *testing.T) {
	invoker := &Invoker{registry: &registry{}}

	// A typed nil invoker function exercises the typed nil rejection path.
	var nilInvokeFunc func()

	tests := []struct {
		name string
		call func()
		want string
	}{
		{
			name: "UntypedNil",
			call: func() { _, _ = invoker.Invoke(nil) },
			want: "expected a function, got nil",
		},
		{
			name: "NonFunctionValue",
			call: func() { _, _ = invoker.Invoke(42) },
			want: "expected a function, got int",
		},
		{
			name: "TypedNilFunction",
			call: func() { _, _ = invoker.Invoke(nilInvokeFunc) },
			want: "expected a non-nil function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPanics(t, tt.want, tt.call)
		})
	}
}

// TestInvokerInvokeValidReturnsError verifies that a valid function never panics:
// an unresolved dependency is surfaced as an error for well-formed input.
func TestInvokerInvokeValidReturnsError(t *testing.T) {
	invoker := &Invoker{registry: &registry{}}

	// A valid function whose dependency is unregistered must return an error.
	values, err := invoker.Invoke(func(dep *testService1) {})
	equal(t, values, []any(nil))
	equal(t, err != nil, true)
}
