[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# Service container for Golang projects

## Features
 
- 🚀 Eager services instantiation with automatic dependencies resolution and optional dependencies support.
- 🛠 Dependency Injection for service factories, avoiding manual fetch through container API.
- 🔄 Reverse-to-Instantiation order for service termination to ensure proper resource release and shutdown. 
- 📣 Events broker for inter-service container-wide communications.
- 🤖 Clean, well-tested and small codebase based on reflection.

## Examples

* [Basic script example](./examples/01_basic_usage/main.go) showing how to do something useful and then exit.
* [Daemon service example](./examples/02_daemon_service/main.go) showing how to launch background services.
* [Events handling example]() showing how to communicate with events.

## Key concepts

* **Service Factory**
* **Service**
* **Service Function**
* **Event Dispatcher**

## Service functions

Sometimes it is redundant to create service object type and implement `Close()` on it, e.g. for `main` glue service.
Instead, it is possible to define factory functions without concrete return type, but defining one function
with a `func() error` interface.

Functions with this signature will be wrapped as a regular object services and started in the background.
The context in argument will be cancelled when the service close initiated by closing the app. The contexts `Done()` chan
can be awaited to catch the service termination. Error value of service function will be used as regular error from `Close()`.

## Builtin services

1. The `gontainer.Events` service gives access to the events broker. It can be used to send and receive events
   inside service container between services or outside from the client code. See `signals` example service.
2. The `context.Context` service gives access to the per-service context, inherited from the root app context.
   This context is cancelled right before the service `Close()` call and not when the entire app was closed.

## Builtin events

1. ContainerClose
2. UnhandledPanic