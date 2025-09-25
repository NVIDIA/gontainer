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
	"sync"
)

// FactoryFunc declares the type for a service factory function.
// A factory function may accept dependencies as input parameters and
// must return exactly one service, optionally followed by an error.
type FactoryFunc any

// Factory declares a service factory definition used by the container to construct services.
//
// A Factory wraps a factory function along with its input/output type information and internal
// state used during service resolution and lifecycle management.
//
// It is created using NewFactory or NewService, and typically registered into the container
// to enable dependency injection and lifecycle control.
type Factory struct {
	// Factory function.
	fn FactoryFunc

	// Factory function name.
	name string

	// Factory function location.
	source string
}

// Name returns factory function name.
func (f *Factory) Name() string {
	return f.name
}

// Source returns factory function source.
func (f *Factory) Source() string {
	return f.source
}

// factory produces internal representation for the factory.
// Separate internal representation is used to let single
// factory instance be used in multiple containers.
func (f *Factory) factory(ctx context.Context) (*factory, error) {
	// Check factory configured.
	if f.fn == nil {
		return nil, errors.New("func is nil")
	}

	// Validate factory type and signature.
	funcType := reflect.TypeOf(f.fn)
	funcValue := reflect.ValueOf(f.fn)
	if funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid type: %s", funcType)
	}

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

	var outType reflect.Type

	// Validate factory output types.
	switch {
	// Factory returns nothing.
	case len(outTypes) == 0:

	// Factory returns only error.
	case len(outTypes) == 1 && isErrorInterface(outTypes[0]):

	// Factory returns exactly one service.
	case len(outTypes) == 1 && !isEmptyInterface(outTypes[0]):
		outType = outTypes[0]

	// Factory returns a service and an error.
	case len(outTypes) == 2 && !isEmptyInterface(outTypes[0]) && isErrorInterface(outTypes[1]):
		outType = outTypes[0]

	// Factory has invalid out signature.
	default:
		return nil, fmt.Errorf("invalid signature: %s", funcType)
	}

	// Prepare cancellable context for the factory services.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	// Prepare registry factory instance.
	return &factory{
		source:    f,
		spawned:   false,
		ctx:       ctx,
		cancel:    cancel,
		funcType:  funcType,
		funcValue: funcValue,
		inTypes:   inTypes,
		outType:   outType,
	}, nil
}

// NewService creates a new service factory that always returns the given singleton value.
//
// This is a convenience helper for registering preconstructed service instances
// as factories. The returned factory produces the same instance on every invocation.
//
// This is useful for registering constants, mocks, or externally constructed values.
//
// Example:
//
//	logger := NewLogger()
//	gontainer.NewService(logger)
func NewService[T any](singleton T) *Factory {
	dataType := reflect.TypeOf(&singleton).Elem()
	factory := &Factory{
		fn:     func() T { return singleton },
		name:   fmt.Sprintf("Service[%s]", dataType),
		source: dataType.PkgPath(),
	}
	return factory
}

// NewFactory creates a new service factory using the provided factory function.
//
// The factory function must be a valid function. It may accept dependencies as input parameters,
// and return one or more service instances, optionally followed by an error as the last return value.
//
// The resulting Factory can be registered in the container.
//
// Example:
//
//	gontainer.NewFactory(func(db *Database) (*Handler, error), gontainer.WithTag("http"))
func NewFactory(factoryFn FactoryFunc) *Factory {
	funcValue := reflect.ValueOf(factoryFn)
	factory := &Factory{
		fn:     factoryFn,
		name:   fmt.Sprintf("Factory[%s]", funcValue.Type()),
		source: getFuncSource(funcValue),
	}
	return factory
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

// factory is the factory internal representation.
type factory struct {
	// Factory source.
	source *Factory

	// Factory spawn mutex.
	spawnMu sync.Mutex

	// Factory spawned mutex.
	spawnedMu sync.RWMutex

	// Factory is spawned.
	spawned bool

	// Factory context value.
	ctx context.Context

	// Factory context cancel.
	cancel context.CancelFunc

	// Factory function type.
	funcType reflect.Type

	// Factory function value.
	funcValue reflect.Value

	// Factory input types.
	inTypes []reflect.Type

	// Factory output type.
	outType reflect.Type

	// Factory output mutex.
	outValueMu sync.RWMutex

	// Factory output value.
	outValue reflect.Value
}

// getSpawned returns factory spawned status in a thread-safe way.
func (f *factory) getSpawned() bool {
	f.spawnedMu.RLock()
	defer f.spawnedMu.RUnlock()
	return f.spawned
}

// setSpawned sets factory spawned status in a thread-safe way.
func (f *factory) setSpawned(value bool) {
	f.spawnedMu.Lock()
	defer f.spawnedMu.Unlock()
	f.spawned = value
}

// getOutValue returns factory output value in a thread-safe way.
func (f *factory) getOutValue() reflect.Value {
	f.outValueMu.RLock()
	defer f.outValueMu.RUnlock()
	return f.outValue
}

// setOutValue sets factory output value in a thread-safe way.
func (f *factory) setOutValue(value reflect.Value) {
	f.outValueMu.Lock()
	defer f.outValueMu.Unlock()
	f.outValue = value
}
