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
	"reflect"
	"runtime"
	"strings"
	"sync"
)

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

// newFactory loads factory function to the internal representation.
func newFactory(
	ctx context.Context,
	name, source string, funcValue reflect.Value,
	getOutType getOutTypeFn, getOutValue getOutValueFn,
	getOutClose getOutCloseFn, getOutError getOutErrorFn,
) (*factory, error) {
	// Prepare function reflect type.
	funcType := funcValue.Type()

	// Index factory input types from the function signature.
	inTypes := make([]reflect.Type, 0, funcType.NumIn())
	for index := 0; index < funcType.NumIn(); index++ {
		inTypes = append(inTypes, funcType.In(index))
	}

	// Index factory output types from the function signature.
	outTypes := make([]reflect.Type, 0, funcType.NumOut())
	for index := 0; index < funcType.NumOut(); index++ {
		outTypes = append(outTypes, funcType.Out(index))
	}

	// Prepare cancellable context for the factory services.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	// Prepare registry factory instance.
	return &factory{
		ctx:       ctx,
		cancel:    cancel,
		name:      name,
		source:    source,
		funcType:  funcType,
		funcValue: funcValue,
		inTypes:   inTypes,
		outTypes:  outTypes,

		// Signature-dependent.
		getOutTypeFn:  getOutType,
		getOutValueFn: getOutValue,
		getOutCloseFn: getOutClose,
		getOutErrorFn: getOutError,
	}, nil
}

// factory is the factory internal representation.
type factory struct {
	// Factory context value.
	ctx context.Context

	// Factory context cancel.
	cancel context.CancelFunc

	// Factory func name.
	name string

	// Factory func source.
	source string

	// Factory spawn mutex.
	spawnMu sync.Mutex

	// Factory is spawned mutex.
	isSpawnedMu sync.RWMutex

	// Factory is spawned.
	isSpawned bool

	// Factory function type.
	funcType reflect.Type

	// Factory function value.
	funcValue reflect.Value

	// Factory input types.
	inTypes []reflect.Type

	// Factory output types.
	outTypes []reflect.Type

	// Factory output mutex.
	outValuesMu sync.RWMutex

	// Factory output values.
	outValues []reflect.Value

	// Factory output type getter.
	getOutTypeFn getOutTypeFn

	// Factory output value getter.
	getOutValueFn getOutValueFn

	// Factory output close getter.
	getOutCloseFn getOutCloseFn

	// Factory output error getter.
	getOutErrorFn getOutErrorFn
}

// getIsSpawned returns factory spawned status in a thread-safe way.
func (f *factory) getIsSpawned() bool {
	f.isSpawnedMu.RLock()
	defer f.isSpawnedMu.RUnlock()
	return f.isSpawned
}

// setIsSpawned sets factory spawned status in a thread-safe way.
func (f *factory) setIsSpawned(value bool) {
	f.isSpawnedMu.Lock()
	defer f.isSpawnedMu.Unlock()
	f.isSpawned = value
}

// getOutValues returns factory output values in a thread-safe way.
func (f *factory) getOutValues() []reflect.Value {
	f.outValuesMu.RLock()
	defer f.outValuesMu.RUnlock()
	return f.outValues
}

// setOutValues sets factory output values in a thread-safe way.
func (f *factory) setOutValues(values []reflect.Value) {
	f.outValuesMu.Lock()
	defer f.outValuesMu.Unlock()
	f.outValues = values
}

// getOutType returns factory output type in a thread-safe way.
func (f *factory) getOutType() reflect.Type {
	return f.getOutTypeFn(f.outTypes)
}

// getOutValue returns factory output value in a thread-safe way.
func (f *factory) getOutValue() reflect.Value {
	if !f.getIsSpawned() {
		return reflect.Value{}
	}

	// Get the factory output value.
	outValues := f.getOutValues()
	return f.getOutValueFn(outValues)
}

// getOutError returns factory output error in a thread-safe way.
func (f *factory) getOutError() error {
	// Get the factory output value.
	outValues := f.getOutValues()
	outValue := f.getOutErrorFn(outValues)

	// Check if the value is valid.
	if !outValue.IsValid() {
		return nil
	}

	// Check if the value is nil.
	if outValue.IsNil() {
		return nil
	}

	// Check if the value is an error.
	if err, ok := outValue.Interface().(error); ok {
		return err
	}

	// Return nil.
	return nil
}

// getOutClose returns factory close function in a thread-safe way.
func (f *factory) getOutClose() func() error {
	// Get the factory closer value.
	outValues := f.getOutValues()
	outValue := f.getOutCloseFn(outValues)

	// Check if the value is valid.
	if !outValue.IsValid() {
		return func() error {
			return nil
		}
	}

	// Check if the value is nil.
	if outValue.IsNil() {
		return func() error {
			return nil
		}
	}

	// Check if the value is a close function.
	if closeFunc, ok := outValue.Interface().(func() error); ok {
		return closeFunc
	}

	// Return a no-op close function.
	return func() error {
		return nil
	}
}

// getOutTypeFn is the function type for getting an output type.
type getOutTypeFn func([]reflect.Type) reflect.Type

// getOutValueFn is the function type for getting an output value.
type getOutValueFn func([]reflect.Value) reflect.Value

// getOutErrorFn is the function type for getting an output error.
type getOutErrorFn func([]reflect.Value) reflect.Value

// getOutCloseFn is the function type for getting a close function.
type getOutCloseFn func([]reflect.Value) reflect.Value
