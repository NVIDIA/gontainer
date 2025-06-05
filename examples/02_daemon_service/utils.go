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

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/NVIDIA/gontainer"
)

// initCloseSignals creates a goroutine to listen for SIGTERM and SIGINT.
func initCloseSignals(container gontainer.Container, errorFn func(err error)) {
	go func() {
		signalsChan := make(chan os.Signal)
		signal.Notify(signalsChan, syscall.SIGTERM, syscall.SIGINT)
		for {
			select {
			case _ = <-signalsChan:
				// Initiate container close.
				if err := container.Close(); err != nil {
					errorFn(err)
				}
				return
			case <-container.Done():
				// Release goroutine.
				return
			}
		}
	}()
}
