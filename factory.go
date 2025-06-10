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
// return zero or more service instances, optionally followed by an error.
// The container validates its signature at runtime using reflection.
//
// Valid example signatures:
//
//	// No dependencies, no produced services.
//	func()
//
//	// No dependencies, one produced service.
//	func() *MyService
//
//	// One dependency, one produced service, an error.
//	func(db *Database) (*Repo, error)
//
//	// Multiple dependencies, multiple produced services, an error.
//	func(log *slog.Logger, db *Database) (*Repo1, *Repo2, error)
//
//	// Multiple dependencies, multiple produced services, no error.
//	func(log *slog.Logger, db *Database) (*Repo1, *Repo2, *Repo3)
//
//	// One optional dependency, one produced service, an error.
//	func(optionalDB gontainer.Optional[*Database]) (*Repo, error)
//
//	// One multiple dependency, one produced service, an error.
//	func(multipleDBs gontainer.Multiple[IDatabase]) (*Repo, error)
type FactoryFunc any

// FactoryMetadata defines a key-value store for attaching metadata to a factory.
//
// Metadata can be used for annotations, tagging, grouping, versioning, or
// integration with external tools. It is populated using `WithMetadata()` option.
type FactoryMetadata map[string]any

// Factory declares a service factory definition used by the container to construct services.
//
// A Factory wraps a factory function along with its metadata, input/output type information,
// and internal state used during service resolution and lifecycle management.
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

	// Factory metadata.
	metadata FactoryMetadata
}

// Name returns factory function name.
func (f *Factory) Name() string {
	return f.name
}

// Source returns factory function source.
func (f *Factory) Source() string {
	return f.source
}

// Metadata returns associated factory metadata.
func (f *Factory) Metadata() FactoryMetadata {
	return f.metadata
}

// factory produces internal representation for the factory.
// Separate internal representation is used to let single
// factory instance be used in multiple containers.
func (f *Factory) factory() (*factory, error) {
	// Check factory configured.
	if f.fn == nil {
		return nil, errors.New("invalid factory func: no func specified")
	}

	// Validate factory type and signature.
	funcType := reflect.TypeOf(f.fn)
	funcValue := reflect.ValueOf(f.fn)
	if funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid factory func: not a function: %s", funcType)
	}

	// Index factory input types from the function signature.
	inTypes := make([]reflect.Type, 0, funcType.NumIn())
	for index := 0; index < funcType.NumIn(); index++ {
		inTypes = append(inTypes, funcType.In(index))
	}

	// Index factory output types from the function signature.
	outTypes := make([]reflect.Type, 0, funcType.NumOut())
	outError := false
	for index := 0; index < funcType.NumOut(); index++ {
		if index != funcType.NumOut()-1 || funcType.Out(index) != errorType {
			// Register regular factory output type.
			outTypes = append(outTypes, funcType.Out(index))
		} else {
			// Register last output index as an error.
			outError = true
		}
	}

	// Prepare cancellable context for the factory services.
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare registry factory instance.
	return &factory{
		source:    f,
		spawned:   false,
		ctx:       ctx,
		cancel:    cancel,
		funcType:  funcType,
		funcValue: funcValue,
		inTypes:   inTypes,
		outTypes:  outTypes,
		outError:  outError,
	}, nil
}

// FactoryOpt defines a functional option for configuring a service factory.
//
// Factory options allow customizing the behavior or metadata of a factory
// at the time of its creation, using functions like WithMetadata, WithTag, etc.
type FactoryOpt func(*Factory)

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
func NewService[T any](singleton T, opts ...FactoryOpt) *Factory {
	dataType := reflect.TypeOf(&singleton).Elem()
	factory := &Factory{
		fn:       func() T { return singleton },
		name:     fmt.Sprintf("Service[%s]", dataType),
		source:   dataType.PkgPath(),
		metadata: FactoryMetadata{},
	}
	for _, opt := range opts {
		opt(factory)
	}
	return factory
}

// NewFactory creates a new service factory using the provided factory function.
//
// The factory function must be a valid function. It may accept dependencies as input parameters,
// and return one or more service instances, optionally followed by an error as the last return value.
//
// Optional configuration can be applied via factory options (`FactoryOpt`), such as providing additional metadata.
//
// The resulting Factory can be registered in the container.
//
// Example:
//
//	gontainer.NewFactory(func(db *Database) (*Handler, error), gontainer.WithTag("http"))
func NewFactory(factoryFn FactoryFunc, opts ...FactoryOpt) *Factory {
	funcValue := reflect.ValueOf(factoryFn)
	factory := &Factory{
		fn:       factoryFn,
		name:     fmt.Sprintf("Factory[%s]", funcValue.Type()),
		source:   getFuncSource(funcValue),
		metadata: FactoryMetadata{},
	}
	for _, opt := range opts {
		opt(factory)
	}
	return factory
}

// WithMetadata adds a custom metadata key-value pair to the factory.
//
// Metadata can be used to attach arbitrary information to a factory,
// such as labels, tags, annotations, or integration-specific flags.
// This data is accessible through the factory’s metadata map at runtime.
//
// Example:
//
//	gontainer.NewFactory(..., gontainer.WithMetadata("version", "v1.2"))
func WithMetadata(key string, value any) FactoryOpt {
	return func(factory *Factory) {
		factory.metadata[key] = value
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

// factory is the factory internal representation.
type factory struct {
	// Factory source.
	source *Factory

	// Factory mutex.
	mutex sync.RWMutex

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

	// Factory output types.
	outTypes []reflect.Type

	// Factory output error.
	outError bool

	// Factory result values.
	outValues []reflect.Value
}

// getSpawned returns factory spawned status in a thread-safe way.
func (f *factory) getSpawned() bool {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.spawned
}

// setSpawned sets factory spawned status in a thread-safe way.
func (f *factory) setSpawned(value bool) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.spawned = value
}

// getOutValues returns factory output values in a thread-safe way.
func (f *factory) getOutValues() []reflect.Value {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.outValues
}

// setOutValues sets factory output values in a thread-safe way.
func (f *factory) setOutValues(values []reflect.Value) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.outValues = values
}
