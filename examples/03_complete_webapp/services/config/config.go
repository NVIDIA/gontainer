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

package config

import (
	"errors"
	"os"
	"reflect"
)

// configTag defines the config tag name.
const configTag = "env"

// NewConfig returns a new Config service.
func NewConfig() *Config {
	return &Config{}
}

// Config is a configuration loader service.
type Config struct{}

// Load loads configuration from environment variables into a struct using the `env` tag.
func (c *Config) Load(structPtr any) error {
	// Reflect the struct pointer.
	targetValue := reflect.ValueOf(structPtr)

	// Allow only a pointer to a struct.
	if targetValue.Type().Kind() != reflect.Ptr || targetValue.Type().Elem().Kind() != reflect.Struct {
		return errors.New("target must be a pointer to a struct")
	}

	// Indirect the struct pointer.
	targetValue = targetValue.Elem()
	targetType := targetValue.Type()

	// Handle every struct field.
	for index := 0; index < targetType.NumField(); index++ {
		// Allow only fields with a config tag set.
		configTagValue := targetType.Field(index).Tag.Get(configTag)
		if configTagValue == "" {
			continue
		}

		// Allow only settable string fields.
		fieldValue := targetValue.Field(index)
		if fieldValue.Kind() != reflect.String || !fieldValue.CanSet() {
			continue
		}

		// Set the field value from the env variable.
		fieldValue.SetString(os.Getenv(configTagValue))
	}

	return nil
}
