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

package logging

// Config is a logger configuration.
type Config struct {
	// Format is the log output format.
	// Valid values are: `text`, `json`.
	Format string `env:"LOG_FORMAT"`

	// Level is the logger output level.
	// Valid values are: `debug`, `info`, `warn`, `error`.
	Level string `env:"LOG_LEVEL"`
}
