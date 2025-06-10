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
	"testing"
)

// TestFactoryLoad tests factory loading.
func TestFactoryLoad(t *testing.T) {
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	opts := WithMetadata("test", "value")
	factory := NewFactory(fun, opts)
	state, err := factory.factory()
	equal(t, err, nil)

	equal(t, factory.metadata["test"], "value")
	equal(t, factory.fn == nil, false)
	equal(t, state.spawned, false)
	equal(t, state.ctx != nil, true)
	equal(t, state.cancel != nil, true)
	equal(t, state.funcType.String(), "func(string, string, string) (int, bool, error)")
	equal(t, state.funcValue.String(), "<func(string, string, string) (int, bool, error) Value>")
	equal(t, fmt.Sprint(state.inTypes), "[string string string]")
	equal(t, fmt.Sprint(state.outTypes), "[int bool]")
	equal(t, state.outError, true)
	equal(t, len(state.outValues), 0)
}

// TestFactoryInfo tests factories info.
func TestFactoryInfo(t *testing.T) {
	type localType struct{}

	localFunc := func(globalType) string {
		return "string"
	}

	tests := []struct {
		name  string
		arg1  *Factory
		want1 string
		want2 string
	}{
		{
			name:  "ServiceLocalType",
			arg1:  NewService(localType{}),
			want1: "Service[gontainer.localType]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "ServiceGlobalType",
			arg1:  NewService(globalType{}),
			want1: "Service[gontainer.globalType]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "FactoryLocalFunc",
			arg1:  NewFactory(localFunc),
			want1: "Factory[func(gontainer.globalType) string]",
			want2: "github.com/NVIDIA/gontainer",
		},
		{
			name:  "FactoryGlobalFunc",
			arg1:  NewFactory(globalFunc),
			want1: "Factory[func(string)]",
			want2: "github.com/NVIDIA/gontainer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1 := tt.arg1.Name()
			if got1 != tt.want1 {
				t.Errorf("Factory.Name() got = %v, want %v", got1, tt.want1)
			}
			got2 := tt.arg1.Source()
			if got2 != tt.want2 {
				t.Errorf("Factory.Source() got = %v, want %v", got2, tt.want2)
			}
		})
	}
}

// TestFactoryMetadata tests factory metadata attachment.
func TestFactoryMetadata(t *testing.T) {
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	var opts []FactoryOpt
	opts = append(opts, WithMetadata("key1", "value1"))
	opts = append(opts, WithMetadata("key2", 123456))
	factory := NewFactory(fun, opts...)

	equal(t, factory.Metadata(), FactoryMetadata{
		"key1": "value1",
		"key2": 123456,
	})
}

type globalType struct{}

func globalFunc(string) {}

// TestSplitFuncName tests splitting of function name.
func TestSplitFuncName(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		want1 string
		want2 string
	}{{
		name:  "SplitPublicPackage",
		arg:   "github.com/NVIDIA/gontainer/app.WithApp.func1",
		want1: "github.com/NVIDIA/gontainer/app",
		want2: "WithApp.func1",
	}, {
		name:  "SplitMainPackage",
		arg:   "main.main.func1",
		want1: "main",
		want2: "main.func1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, got2 := splitFuncName(tt.arg)
			if got1 != tt.want1 {
				t.Errorf("splitFuncName() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("splitFuncName() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
