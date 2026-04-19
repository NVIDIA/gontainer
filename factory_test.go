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
	"strings"
	"testing"
)

// TestFactoryLoad tests factory loading.
func TestFactoryLoad(t *testing.T) {
	fun := func(a, b, c string) (int, error) {
		return 100500, nil
	}

	option := NewFactory(fun)
	registry := &registry{}
	equal(t, option.apply(registry), nil)
	factory := registry.factories[0]

	equal(t, factory.funcType.String(), "func(string, string, string) (int, error)")
	equal(t, factory.funcValue.String(), "<func(string, string, string) (int, error) Value>")
	equal(t, fmt.Sprint(factory.inTypes), "[string string string]")
	equal(t, fmt.Sprint(factory.outTypes), "[int error]")
	equal(t, fmt.Sprint(factory.getOutType()), "int")
	equal(t, factory.isSpawned, false)
	equal(t, factory.outValues, []reflect.Value(nil))
	equal(t, factory.getOutValue().IsValid(), false)
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
	}{
		{
			name:  "ServiceLocalType",
			arg1:  NewService(localType{}),
			want1: "Service[gontainer.localType]",
		},
		{
			name:  "ServiceGlobalType",
			arg1:  NewService(globalType{}),
			want1: "Service[gontainer.globalType]",
		},
		{
			name:  "FactoryLocalFunc",
			arg1:  NewFactory(localFunc),
			want1: "Factory[func(gontainer.globalType) string]",
		},
		{
			name:  "FactoryGlobalFunc",
			arg1:  NewFactory(globalFunc),
			want1: "Factory[func(string) bool]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &registry{}
			equal(t, tt.arg1.apply(registry), nil)

			factory := registry.factories[0]
			equal(t, factory.name, tt.want1)
			// Source is now a clickable "<file>:<line>" location.
			if !strings.Contains(factory.source, ".go:") {
				t.Fatalf("expected file:line source, got %q", factory.source)
			}
		})
	}
}

type globalType struct{}

func globalFunc(string) bool {
	return true
}

// TestSplitFuncName tests splitting of function name.
func TestSplitFuncName(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		want1 string
		want2 string
	}{{
		name:  "SplitPublicPackage",
		arg:   "github.com/NVIDIA/gontainer/v2/app.WithApp.func1",
		want1: "github.com/NVIDIA/gontainer/v2/app",
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
