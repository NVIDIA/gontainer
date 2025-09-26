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
