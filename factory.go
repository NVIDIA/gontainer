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

// FactoryInstMode configures services instantiation mode for the factory.
type factoryMode int

// A factory can produce services at once or always, depending on the mode.
const (
	factoryModeSingleton factoryMode = iota
	factoryModeTransient
)

// Factory declares a service factory definition used by the container to construct services.
//
// A Factory wraps a factory function along with its metadata, input/output type information,
// and internal state used during service resolution and lifecycle management.
//
// It is created using NewFactory or NewService, and typically registered into the container
// to enable dependency injection and lifecycle control.
type Factory struct {
	// Factory function.
	factoryFunc FactoryFunc

	// Factory function name.
	factoryName string

	// Factory function location.
	factorySource string

	// Factory metadata.
	factoryMetadata FactoryMetadata

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

	// Factory is loaded.
	factoryLoaded bool

	// Factory mode.
	// Singleton or transient.
	factoryMode factoryMode

	// Factory mutex.
	// Only for singleton factories.
	factoryMutex sync.RWMutex

	// Factory is spawned.
	// Only for singleton factories.
	factorySpawned bool

	// Factory context value.
	// Only for singleton factories.
	factoryCtx context.Context

	// Factory context cancel.
	// Only for singleton factories.
	factoryCtxCancel context.CancelFunc

	// Factory result values.
	// Only for singleton factories.
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
func (f *Factory) load() error {
	if f.factoryLoaded {
		return errors.New("invalid factory func: already loaded")
	}

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
	for index := 0; index < f.factoryType.NumOut(); index++ {
		if index != f.factoryType.NumOut()-1 || f.factoryType.Out(index) != errorType {
			// Register regular factory output type.
			f.factoryOutTypes = append(f.factoryOutTypes, f.factoryType.Out(index))
		} else {
			// Register last output index as an error.
			f.factoryOutError = true
		}
	}

	// Prepare data for singleton factories.
	if f.factoryMode == factoryModeSingleton {
		ctx, cancel := context.WithCancel(context.Background())
		f.factoryCtx, f.factoryCtxCancel = ctx, cancel
	}

	// Save the factory load status.
	f.factoryLoaded = true
	return nil
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
		factoryFunc:     func() T { return singleton },
		factoryName:     fmt.Sprintf("Service[%s]", dataType),
		factorySource:   dataType.PkgPath(),
		factoryMetadata: FactoryMetadata{},
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
		factory.factoryMetadata[key] = value
	}
}

// WithSingletonMode sets the factory to produce new instances only once
// when the service is requested at first time (lazy, without container start)
// or when the factory is invoked on the eager container start call.
//
// Instances are managed by the container lifecycle and will be closed by
// the container when it will be closed.
//
// This is the default behavior of factories in the container.
func WithSingletonMode() FactoryOpt {
	return func(factory *Factory) {
		factory.factoryMode = factoryModeSingleton
	}
}

// WithTransientMode sets the factory to produce new instances every time:
//
//   - for every dependency injection;
//   - for every resolution via resolver service;
//   - for every resolution via invoker service.
//
// Instances are not managed by the container lifecycle and should be closed by user.
func WithTransientMode() FactoryOpt {
	return func(factory *Factory) {
		factory.factoryMode = factoryModeTransient
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
