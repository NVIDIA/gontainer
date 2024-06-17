package gontainer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestContainerLifecycle tests container lifecycle.
func TestContainerLifecycle(t *testing.T) {
	factoryStarted := atomic.Bool{}
	serviceStarted := atomic.Bool{}
	serviceClosed := atomic.Bool{}

	container, err := New(
		NewService(float64(100500)),
		NewFactory(func() string { return "string" }),
		NewFactory(func() (int, int64) { return 123, 456 }),
		NewFactory(func(ctx context.Context, dep1 float64, dep2 string, dep3 int) any {
			equal(t, dep1, float64(100500))
			equal(t, dep2, "string")
			equal(t, dep3, 123)
			factoryStarted.Store(true)
			return func() error {
				serviceStarted.Store(true)
				<-ctx.Done()
				serviceClosed.Store(true)
				return nil
			}
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Assert factories and services.
	equal(t, len(container.Factories()), 7)
	equal(t, len(container.Services()), 0)

	// Start all factories in the container.
	equal(t, container.Start(), nil)
	equal(t, factoryStarted.Load(), true)
	equal(t, serviceClosed.Load(), false)

	// Assert factories and services.
	equal(t, len(container.Factories()), 7)
	equal(t, len(container.Services()), 8)

	// Let factory function start executing in the background.
	time.Sleep(time.Millisecond)

	equal(t, serviceStarted.Load(), true)
	equal(t, serviceClosed.Load(), false)

	// Close all factories in the container.
	equal(t, container.Close(), nil)
	equal(t, serviceClosed.Load(), true)

	// Assert context is closed.
	<-container.Done()

	// Assert factories and services.
	equal(t, len(container.Factories()), 7)
	equal(t, len(container.Services()), 0)
}
