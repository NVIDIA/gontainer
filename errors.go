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
	"fmt"
	"reflect"
	"strings"
)

// ErrFactoryTypeDuplicated declares service duplicated error.
var ErrFactoryTypeDuplicated = errors.New("factory type duplicated")

// ErrFactoryReturnedError declares factory returned error.
var ErrFactoryReturnedError = errors.New("factory returned error")

// ErrNoEntrypointsProvided declares no entrypoints provided error.
var ErrNoEntrypointsProvided = errors.New("no entrypoints provided")

// ErrEntrypointReturnedError declares entrypoint returned error.
var ErrEntrypointReturnedError = errors.New("entrypoint returned error")

// ErrDependencyNotResolved declares service not resolved error.
var ErrDependencyNotResolved = errors.New("dependency not resolved")

// ErrCircularDependency declares a circular dependency error.
var ErrCircularDependency = errors.New("circular dependency")

// formatFactoryFrame renders a single factory or entrypoint as one traceback frame.
func formatFactoryFrame(f *factory) string {
	var sb strings.Builder
	sb.WriteString("\n  ")
	if f.kind == kindEntrypoint {
		sb.WriteString("Entrypoint")
	} else {
		sb.WriteString("Factory for ")
		sb.WriteString(f.getOutType().String())
	}
	if f.source != "" {
		sb.WriteString("\n    at ")
		sb.WriteString(f.source)
	}
	return sb.String()
}

// newDependencyNotResolvedError reports that no factory could satisfy missing for requester.
func newDependencyNotResolvedError(requester *factory, missing reflect.Type) error {
	tail := "\n\nTraceback:"
	if requester != nil {
		tail += formatFactoryFrame(requester)
	}
	return fmt.Errorf("%w: %s%s", ErrDependencyNotResolved, missing, tail)
}

// newFactoryTypeDuplicatedError reports that more than one factory produces the same output type.
func newFactoryTypeDuplicatedError(f *factory) error {
	return fmt.Errorf("%w: %s\n\nTraceback:%s", ErrFactoryTypeDuplicated, f.getOutType(), formatFactoryFrame(f))
}

// newCircularDependencyError reports a cycle in the dependency graph starting at f.
func newCircularDependencyError(f *factory) error {
	return fmt.Errorf("%w\n\nTraceback:%s", ErrCircularDependency, formatFactoryFrame(f))
}

// newFactoryResolveFailedError appends f as an outer frame to an already-rendered resolve error.
func newFactoryResolveFailedError(f *factory, err error) error {
	return fmt.Errorf("%w%s", err, formatFactoryFrame(f))
}

// newFactoryReturnedErrorError wraps a raw user error returned by a service factory and opens a Traceback section.
func newFactoryReturnedErrorError(f *factory, err error) error {
	return fmt.Errorf("%w\n\nTraceback:%s%.0w", err, formatFactoryFrame(f), ErrFactoryReturnedError)
}

// newEntrypointReturnedErrorError wraps a raw user error returned by an entrypoint and opens a Traceback section.
func newEntrypointReturnedErrorError(f *factory, err error) error {
	return fmt.Errorf("%w\n\nTraceback:%s%.0w", err, formatFactoryFrame(f), ErrEntrypointReturnedError)
}

// newFactoryCloseFailedError wraps a raw close-callback error under a Source section with f as the sole frame.
func newFactoryCloseFailedError(f *factory, err error) error {
	return fmt.Errorf("%w\n\nSource:%s", err, formatFactoryFrame(f))
}

// errorGroup is a group of independent errors, separated by a blank line when rendered.
type errorGroup []error

// Error renders each child error separated by a blank line.
func (g errorGroup) Error() string {
	parts := make([]string, 0, len(g))
	for _, err := range g {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n\n")
}

// Unwrap exposes the underlying errors so errors.Is can walk into each child independently.
func (g errorGroup) Unwrap() []error {
	return g
}
