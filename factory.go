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
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// FactoryFunc declares factory function.
type FactoryFunc any

// FactoryMetadata declares factory metadata.
type FactoryMetadata map[string]any

// Factory declares service factory.
type Factory struct {
	// Factory function.
	factoryFunc FactoryFunc

	// Factory function name.
	factoryName string

	// Factory function location.
	factorySource string

	// Factory metadata.
	factoryMetadata FactoryMetadata

	// Factory is loaded.
	factoryLoaded bool

	// Factory is spawned.
	factorySpawned bool

	// Factory context value.
	factoryCtx context.Context

	// Factory context cancel.
	ctxCancel context.CancelFunc

	// Factory function type.
	factoryType reflect.Type

	// Factory function value.
	factoryValue reflect.Value

	// Factory input types.
	factoryInTypes []reflect.Type

	// Factory output types.
	factoryOutTypes []reflect.Type

	// Factory output error.
	factoryOutError bool

	// Factory result values.
	factoryOutValues []reflect.Value
}

// Name returns factory function name.
func (f *Factory) Name() string {
	return f.factoryName
}

// Source returns factory function source.
func (f *Factory) Source() string {
	return f.factorySource
}

// Metadata returns associated factory metadata.
func (f *Factory) Metadata() FactoryMetadata {
	return f.factoryMetadata
}

// load initializes factory definition internal values.
func (f *Factory) load(ctx context.Context) error {
	if f.factoryLoaded {
		return errors.New("invalid factory func: already loaded")
	}

	// Prepare cancellable context for the factory services.
	f.factoryCtx, f.ctxCancel = context.WithCancel(ctx)

	// Check factory configured.
	if f.factoryFunc == nil {
		return errors.New("invalid factory func: no func specified")
	}

	// Validate factory type and signature.
	f.factoryType = reflect.TypeOf(f.factoryFunc)
	f.factoryValue = reflect.ValueOf(f.factoryFunc)
	if f.factoryType.Kind() != reflect.Func {
		return fmt.Errorf("invalid factory func: not a function: %s", f.factoryType)
	}

	// Index factory input types from the function signature.
	f.factoryInTypes = make([]reflect.Type, 0, f.factoryType.NumIn())
	for index := 0; index < f.factoryType.NumIn(); index++ {
		f.factoryInTypes = append(f.factoryInTypes, f.factoryType.In(index))
	}

	// Index factory output types from the function signature.
	f.factoryOutTypes = make([]reflect.Type, 0, f.factoryType.NumOut())
	f.factoryOutValues = make([]reflect.Value, 0, f.factoryType.NumOut())
	for index := 0; index < f.factoryType.NumOut(); index++ {
		if index != f.factoryType.NumOut()-1 || f.factoryType.Out(index) != errorType {
			// Register regular factory output type.
			f.factoryOutTypes = append(f.factoryOutTypes, f.factoryType.Out(index))
		} else {
			// Register last output index as an error.
			f.factoryOutError = true
		}
	}

	// Save the factory load status.
	f.factoryLoaded = true
	return nil
}

// FactoryOpt defines factory option.
type FactoryOpt func(*Factory)

// NewService creates new service factory with predefined service.
func NewService[T any](singleton T) *Factory {
	dataType := reflect.TypeOf(&singleton).Elem()
	return &Factory{
		factoryFunc:     func() T { return singleton },
		factoryName:     fmt.Sprintf("Service[%s]", dataType),
		factorySource:   dataType.PkgPath(),
		factoryMetadata: FactoryMetadata{},
	}
}

// NewFactory creates new service factory with factory func.
func NewFactory(factoryFn any, opts ...FactoryOpt) *Factory {
	funcValue := reflect.ValueOf(factoryFn)
	factory := &Factory{
		factoryFunc:     factoryFn,
		factoryName:     fmt.Sprintf("Factory[%s]", funcValue.Type()),
		factorySource:   getFuncSource(funcValue),
		factoryMetadata: FactoryMetadata{},
	}
	for _, opt := range opts {
		opt(factory)
	}
	return factory
}

// WithMetadata adds metadata to the factory.
func WithMetadata(key string, value any) FactoryOpt {
	return func(factory *Factory) {
		factory.factoryMetadata[key] = value
	}
}

// getFuncSource returns func source path.
func getFuncSource(funcValue reflect.Value) string {
	fullFuncName := runtime.FuncForPC(funcValue.Pointer()).Name()
	funcPackage, _ := splitFuncName(fullFuncName)
	return funcPackage
}

// splitFuncName splits specified func name to package and a name.
func splitFuncName(funcFullName string) (string, string) {
	// Split the full function name with package by dots.
	fullNameChunks := strings.Split(funcFullName, ".")
	if len(fullNameChunks) < 2 {
		return "", funcFullName
	}

	// Find the index of the last element containing "/".
	lastPackageChunkIndex := len(fullNameChunks) - 1
	for ; lastPackageChunkIndex >= 0; lastPackageChunkIndex-- {
		// Is this chunk the rightest part of a package name with dots?
		if strings.Contains(fullNameChunks[lastPackageChunkIndex], "/") {
			break
		}
	}

	// If the name contains no package path.
	if lastPackageChunkIndex == -1 {
		packageName := fullNameChunks[0]
		funcName := strings.Join(fullNameChunks[1:], ".")
		return packageName, funcName
	}

	// Prepare package name and function name.
	packageName := strings.Join(fullNameChunks[:lastPackageChunkIndex+1], ".")
	funcName := strings.Join(fullNameChunks[lastPackageChunkIndex+1:], ".")
	return packageName, funcName
}
