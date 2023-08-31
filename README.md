[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# Simple Service Container for Golang projects

Service container features.

* Eager (non-lazy) services instantiation.
* Clean but powerful module interface.
* Support for optional service dependencies.
* Event broker for inter-service communication.
* Events for unhandled panics in services. 

## Creating new services

Every service in service container is produced by a service factory produced by `gontainer.NewFactory()`.
Definition of a simple service can look [like the following](./examples/example.go).

```go
return gontainer.NewFactory(
    gontainer.WithFactory(
        func() (Config, error) {
            // Prepare service instance.
            instance, err := New(name)
            if err != nil {
                return nil, err
            }
            // Publish created instance.
            return instance, nil
        },
    ),
    gontainer.WithHelp(
        func() string {
            return `Help for the service.`
        },
    ),
)
```

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
