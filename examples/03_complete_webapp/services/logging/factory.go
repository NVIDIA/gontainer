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

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/NVIDIA/gontainer"
	confmod "github.com/NVIDIA/gontainer/examples/03_complete_webapp/services/config"
)

// WithSlogLogger returns a factory for the slog logger.
func WithSlogLogger() *gontainer.Factory {
	return gontainer.NewFactory(
		func(confsvc *confmod.Config, events gontainer.Events) (*slog.Logger, error) {
			// Prepare logger config.
			config := Config{}
			if err := confsvc.Load(&config); err != nil {
				return nil, fmt.Errorf("failed to load logger config: %w", err)
			}

			// Prepare logger log level
			var level slog.Level
			switch config.Level {
			case "debug":
				level = slog.LevelDebug
			case "info":
				level = slog.LevelInfo
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			default:
				level = slog.LevelError
			}

			// Prepare handler options.
			options := &slog.HandlerOptions{Level: level}
			output := os.Stdout

			// Prepare logger handler.
			var handler slog.Handler
			switch config.Format {
			case "json":
				handler = slog.NewJSONHandler(output, options)
			case "text":
				fallthrough
			default:
				handler = slog.NewTextHandler(output, options)
			}

			// Create new logger instance.
			logger := slog.New(handler)
			loggerWithTag := logger.With("service", "logger")

			// Log container starting message.
			loggerWithTag.Info("Starting service container")

			// Register container started event handler.
			events.Subscribe(gontainer.ContainerStarted, func(err error) {
				if err != nil {
					loggerWithTag.Error("Failed to start service container", slog.Any("error", err))
				} else {
					loggerWithTag.Info("Service container started")
				}
			})

			// Register container closing event handler.
			events.Subscribe(gontainer.ContainerClosing, func() {
				loggerWithTag.Info("Closing service container")
			})

			// Register container closed event handler.
			events.Subscribe(gontainer.ContainerClosed, func(err error) {
				if err != nil {
					loggerWithTag.Error("Failed to close service container", "error", err)
				} else {
					loggerWithTag.Info("Service container closed")
				}
			})

			// Create new logger instance.
			return logger, nil
		},
	)
}
