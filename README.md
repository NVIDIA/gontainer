[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GoDoc](https://pkg.go.dev/badge/github.com/NVIDIA/gontainer)](https://pkg.go.dev/github.com/NVIDIA/gontainer)
![Test](https://github.com/NVIDIA/gontainer/actions/workflows/go.yml/badge.svg)
[![Report](https://goreportcard.com/badge/github.com/NVIDIA/gontainer)](https://goreportcard.com/report/github.com/NVIDIA/gontainer)

# Gontainer

Dependency injection service container for Golang projects.
<p align="center"><img src="splash.png" width="400"/></p>

## Features

- 🚀 Eager or lazy services instantiation with automatic dependencies resolution and optional dependencies support.
- 🛠 Automatic injection of dependencies for service factories, avoiding manual fetch through container API.
- 🔄 Automatic reverse-to-instantiation order for service termination to ensure proper resource release and shutdown.
- 📣 Built-in events broker service for inter-service container-wide communications and loose coupling between services.
- 🤖 Clean and tested implementation based on reflection and generics. No external dependencies.

## Examples

* [Console command example](./examples/01_console_command/main.go) – demonstrates how to build a simple console command. It shows how to use `Resolver` and `Invoker` services to organize the application entry point in a run-and-exit style.
* [Daemon service example](./examples/02_daemon_service/main.go) – demonstrates how to maintain background services. It shows how to organize a daemon entry point and wait for graceful shutdown by subscribing to OS termination signals.
* [Complete webapp example](./examples/03_complete_webapp/main.go) – demonstrates how to organize applications consists of multiple services. It provides basic config service, handles logging, setups HTTP server and initiates two endpoints.
  Here is the log of the complete webapp example launch.
  ```
  time=2025-06-05T15:19:48.373+02:00 level=INFO msg="Starting service container" service=logger
  time=2025-06-05T15:19:48.373+02:00 level=INFO msg="Configuring app endpoints" service=app
  time=2025-06-05T15:19:48.373+02:00 level=INFO msg="Configuring health endpoints" service=app
  time=2025-06-05T15:19:48.373+02:00 level=INFO msg="Starting HTTP server" service=http address=127.0.0.1:8080
  time=2025-06-05T15:19:48.374+02:00 level=INFO msg="Service container started" service=logger
  ----------------- Application was started and now accepts HTTP requests -----------------
  time=2025-06-05T15:19:54.716+02:00 level=INFO msg="Serving home page" service=app remote-addr=127.0.0.1:62640
  time=2025-06-05T15:20:01.405+02:00 level=INFO msg="Serving health check" service=app remote-addr=127.0.0.1:62640
  ----------------- CTRL+C was pressed or a TERM signal was sent to the process -----------------
  time=2025-06-05T15:20:04.061+02:00 level=INFO msg="Closing service container" service=logger
  time=2025-06-05T15:20:04.061+02:00 level=INFO msg="Closing HTTP server" service=http
  time=2025-06-05T15:20:04.061+02:00 level=INFO msg="Service container closed" service=logger
  ```

## Quick Start

1. Define an example service.
    ```go
    // MyService performs some crucial tasks.
    type MyService struct{}

    // SayHello outputs a friendly greeting.
    func (s *MyService) SayHello(name string) {
        log.Println("Hello,", name)
    }
   ```
2. Define a service factory.
   ```go
   func NewMyService() *MyService {
      return new(MyService)
   }
   ```
3. Register service factories in the container.
   ```go
   container, err := gontainer.New(
      // Define MyService factory in the container.
      gontainer.NewFactory(NewMyService),
   
      // Here we can define additional services depending on `*MyService`.
      // All dependencies are declared using factory function args.
      gontainer.NewFactory(func(service *MyService) {
         service.SayHello("Username")
      }),
   )
   if err != nil {
      log.Fatalf("Failed to init service container: %s", err)
   }
   ```
5. Start the container and launch all factories.
   ```go
   if err := container.Start(); err != nil {
      log.Fatalf("Failed to start service container: %s", err)
   }
   ```
6. Alternatively to eager start with a `Start()` call it is possible to use `Resolver` or `Invoker` service. They will instantiate only the explicitly requested services and their transitive dependencies.
   ```go
   var myService *MyService
   if err := container.Resolver().Resolve(&MyService); err != nil {
       log.Fatalf("Failed to resolve dependency: %s", err)
   }
   myService.DoSomething()
   ```
   or
   ```go
   if err := container.Invoker().Invoke(func(myService *MyService) {
       myService.DoSomething()
   }); err != nil {
       log.Fatalf("Failed to invoke a function: %s", err)
   }
   ```
   `Resolver` and `Invoker` could also serve as an entry point to the application, especially when it's a simple console tool that runs a task and exits.
   The [console command example](./examples/01_console_command/main.go) demonstrates this approach.

## Key Concepts

### Service Factories

The **Service Factory** is a key component of the service container, serving as a mechanism for creating service instances.
A service factory is essentially a function that accepts other services as arguments and returns one or more service instances,
optionally along with an error. Using service factory signature, the service container will resolve and spawn all dependency
services using reflection and fail, if there are unresolvable dependencies.

```go
// MyServiceFactory is an example of a service factory.
func MyServiceFactory( /* service dependencies */) *MyService {
   // Initialize service instance.
   return new(MyService)
}

// MyServiceFactory depends on two services.
func MyServiceFactory(svc1 MyService1, svc2 MyService2) MyService {...}

// MyServiceFactory provides two services.
func MyServiceFactory() (MyService1, MyService2) {...}

// MyServiceFactory provides two services and return an error.
func MyServiceFactory() (MyService1, MyService2, error) {...}

// MyServiceFactory returns only an error.
func MyServiceFactory() error {...}

// MyServiceFactory provides nothing.
func MyServiceFactory() {...}
```

The factory function's role is to perform any necessary initializations and return a fully-configured service instance
to the container.

There are several predefined by container service types that may be used as a dependencies in the factory arguments.

1. The `context.Context` service provides the per-service context, inherited from the background context.
   This context is cancelled right before the service's `Close()` call and intended to be used with service functions.
2. The `gontainer.Events` service provides the events broker. It can be used to send and receive events
   inside service container between services or outside from the client code.
3. The `gontainer.Resolver` service provides a service to resolve dependencies dynamically.
4. The `gontainer.Invoker` service provides a service to invoke functions dynamically.

In addition, there are several generic types allowing to declare dependencies on a type.

#### Optional Dependency Declaration

The `gontainer.Optional[T]` type allows to depend on a type that may or may not be present.
For example, when developing a factory that uses a telemetry service, this type can be used if the service
is registered in the container. If the telemetry service is not registered, this is not considered an error,
and telemetry initialization can be skipped in the factory.

```go
// MyServiceFactory optionally depends on the service.
func MyServiceFactory(optService1 gontainer.Optional[MyService1]) {
    // Get will not produce any error if the MyService1 is not registered
    // in the container: it will return zero value for the service type.
    service := optSvc1.Get()
}
```

#### Multiple Dependencies Declaration

The `gontainer.Multiple[T]` type allows retrieval of all services that match the type `T`. This feature is 
intended to be used when providing concrete service types from multiple factories (e.g., struct pointers like
`*passwordauth.Provider`, `*tokenauth.Provider`) and depending on them as services `Multiple[IProvider]`.
In this case, the length of the `services` slice could be in the range `[0, N]`.

If a concrete non-interface type is specified in `T`, the slice can have at most one element. 
The container restricts the registration of the same non-interface type more than once.

```go
// MyServiceFactory depends on all services implementing the interface.
func MyServiceFactory(servicesSlice gontainer.Multiple[MyInterface]) {
    for _, service := range servicesSlice {
        service.DoSomething()
    }
}
```

### Services

A service is a building block of the application, created and managed by a Service Factory. 
The lifetime of a service is tied to the lifetime of the entire container for a singletons
and should be managed by the client code if the factory configured in a transient mode.

A service may optionally implement a `Close() error` or just `Close()` method, which is called when the container is shutting down.
The `Close` call is synchronous: remaining services will not be closed until this method returns.

```go
// MyService defines example service.
type MyService struct {}

// SayHello is service domain method example. 
func (s *MyService) SayHello(name string) {
    fmt.Println("Hello,", name)
}

// Close is an optional method called from container's Close(). 
func (s *MyService) Close() error {
   // Synchronous cleanup logic here.
   return nil
}
```

### Service Functions

The **Service Function** is a specialized form of service optimized for simpler tasks. Instead of returning a concrete
type object or an interface, the service factory returns a function that conforms to `func() error` or `func()` type.

The function serves two primary roles:

* It encapsulates the behavior to execute when the container starts asynchronously to the `Start()` method.
* It returns an error, which is treated as if it were returned by a conventional `Close()` method.

```go
// MyServiceFactory is an example of a service function usage.
func MyServiceFactory(ctx context.Context) func() error {
    return func() error {
        // Await its order in container close.
        <-ctx.Done()
      
        // Return nil from the `service.Close()`.
        return nil
    }
}
```

In this design, the factory function is responsible for receiving the context. This context is canceled when the service
needs to close, allowing the function to terminate gracefully.

Errors returned by the function are processed as if they were errors returned by a standard `Close()` method to the container.
This means the container will synchronously wait until a service function returns an error or nil before closing the next services.

### Events Broker

The **Events Broker** is an additional part of the service container architecture. It facilitates communication between services
without them having to be directly aware of each other. The Events Broker works on a publisher-subscriber model, enabling services
to publish events to, and subscribe to events from, a centralized broker.

This mechanism allows services to remain decoupled while still being able to interact through a centralized medium.
In particular, the `gontainer.Events` service provides an interface to the events broker and can be injected as a dependency in any service factory.

#### Triggering Events

To trigger an event, use the `Trigger()` method. Create an event using `NewEvent()` and pass the necessary arguments:

```go
events.Trigger(gontainer.NewEvent("Event1", event, arguments, here))
```

#### Subscribing to Events

To subscribe to an event, use the `Subscribe()` method. Two types of handler functions are supported:

* A function that accepts a variable number of any-typed arguments:
  ```go
  events.Subscribe("Event1", func(args ...any) {
      // Handle the event with args slice.
  })
  ```
* A function that accepts concrete argument types:
  ```go
  ev.Subscribe("Event1", func(x string, y int, z bool) {
      // Handle the event with specific args.
  })
  ```
  - The **number of arguments** in the event and the handler can differ because handlers are designed to be flexible and can process varying numbers and types of arguments, allowing for greater versatility in handling different event scenarios.
  - The **types of arguments** in the event and the handler must be assignable. Otherwise, an error will be returned from a `Trigger()` call.
  
Every handler function could return an `error` which will be joined and returned from `Trigger()` call.

### Container Interface

```go
// Container defines service container interface.
type Container interface {
    // Start initializes every service in the container.
    Start() error

    // Close closes service container with all services.
    // Blocks invocation until the container is closed.
    Close() error

    // Done is closing after closing of all services.
    Done() <-chan struct{}

    // Factories returns all defined factories.
    Factories() []*Factory

    // Services returns all spawned services.
    Services() []any

    // Events returns events broker instance.
    Events() Events

    // Resolver returns service resolver instance.
    // If container is not started, only requested services
    // will be spawned on `resolver.Resolve(...)` call.
    Resolver() Resolver

    // Invoker returns function invoker instance.
    // If container is not started, only requested services
    // will be spawned to invoke the func.
    Invoker() Invoker
}
```

### Container Events

The service container emits several events during its lifecycle:

| Event               | Description                                                                   |
|---------------------|-------------------------------------------------------------------------------|
| `ContainerStarting` | Emitted when the container's start method is invoked.                         |
| `ContainerStarted`  | Emitted when the container's start method has completed.                      |
| `ContainerClosing`  | Emitted when the container's close method is invoked.                         |
| `ContainerClosed`   | Emitted when the container's close method has completed.                      |
| `UnhandledPanic`    | Emitted when a panic occurs during container initialization, start, or close. |

### Container Errors

The service container may return the following errors, which can be checked using `errors.Is`:

| Error                       | Description                                                                           |
|-----------------------------|---------------------------------------------------------------------------------------|
| `ErrFactoryReturnedError`   | Occurs when the factory function returns an error during invocation.                  |
| `ErrServiceNotResolved`     | Occurs when resolving a service fails due to an unregistered service type.            |
| `ErrServiceDuplicated`      | Occurs when a service type duplicate found during the initialization procedure.       |
| `ErrCircularDependency`     | Occurs when a circular dependency found during the initialization procedure.          |
| `ErrHandlerArgTypeMismatch` | Occurs when an event handler's arguments do not match the event's expected arguments. |
