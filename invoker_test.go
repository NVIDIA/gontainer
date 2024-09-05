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
	"testing"
)

// TestInvokerService tests invoker service.
func TestInvokerService(t *testing.T) {
	tests := []struct {
		name    string
		haveFn  any
		wantFn  func(t *testing.T, value InvokeResult)
		wantErr bool
	}{
		{
			name:   "ReturnNothing",
			haveFn: func(var1 string, var2 int) {},
			wantFn: func(t *testing.T, value InvokeResult) {
				equal(t, len(value.Values()), 0)
				equal(t, value.Error(), nil)
			},
			wantErr: false,
		},
		{
			name: "ReturnValuesNoError",
			haveFn: func(var1 string, var2 int) (string, int, bool) {
				return var1 + "-X", var2 + 100, true
			},
			wantFn: func(t *testing.T, value InvokeResult) {
				equal(t, len(value.Values()), 3)
				equal(t, value.Values()[0], "string-X")
				equal(t, value.Values()[1], 223)
				equal(t, value.Values()[2], true)
				equal(t, value.Error(), nil)
			},
			wantErr: false,
		},
		{
			name: "ReturnNoValuesWithError",
			haveFn: func(var1 string, var2 int) (string, int, error) {
				return var1 + "-X", var2 + 100, errors.New("failed")
			},
			wantFn: func(t *testing.T, value InvokeResult) {
				equal(t, len(value.Values()), 3)
				equal(t, value.Values()[0], "string-X")
				equal(t, value.Values()[1], 223)
				equal(t, value.Values()[2].(error).Error(), "failed")
				equal(t, value.Error().Error(), "failed")
				equal(t, value.Error(), value.Values()[2])
			},
			wantErr: false,
		},
		{
			name: "ReturnMultipleError",
			haveFn: func(var1 string, var2 int) (error, error, error) {
				return nil, errors.New("error-1"), errors.New("error-2")
			},
			wantFn: func(t *testing.T, value InvokeResult) {
				equal(t, len(value.Values()), 3)
				equal(t, value.Values()[0], nil)
				equal(t, value.Values()[1].(error).Error(), "error-1")
				equal(t, value.Values()[2].(error).Error(), "error-2")
				equal(t, value.Error().Error(), "error-2")
				equal(t, value.Error(), value.Values()[2])
			},
			wantErr: false,
		},
		{
			name:   "ReturnNilError",
			haveFn: func(var1 string, var2 int) error { return nil },
			wantFn: func(t *testing.T, value InvokeResult) {
				equal(t, len(value.Values()), 1)
				equal(t, value.Values()[0], nil)
				equal(t, value.Error(), nil)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container, err := New(
				NewFactory(func() string { return "string" }),
				NewFactory(func() int { return 123 }),
			)
			equal(t, err, nil)
			equal(t, container == nil, false)
			defer func() {
				equal(t, container.Close(), nil)
			}()

			result, err := container.Invoker().Invoke(tt.haveFn)
			if (err != nil) != tt.wantErr {
				t.Errorf("Invoke() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantFn != nil {
				tt.wantFn(t, result)
			}
		})
	}
}
