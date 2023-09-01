package gontainer

import (
	"context"
	"testing"
	"time"
)

// TestContainerLifecycle tests container lifecycle.
func TestContainerLifecycle(t *testing.T) {
	factoryStarted := false
	serviceStarted := false
	serviceClosed := false

	container, err := New(
		NewFactory(func() string { return "string" }),
		NewFactory(func() int { return 123 }),
		NewFactory(func(ctx context.Context, dep1 string, dep2 int) any {
			factoryStarted = true
			return func() error {
				serviceStarted = true
				<-ctx.Done()
				serviceClosed = true
				return nil
			}
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Start all factories in the container.
	equal(t, container.Start(), nil)
	equal(t, factoryStarted, true)
	equal(t, serviceClosed, false)

	// Let async service function launch.
	time.Sleep(time.Millisecond)
	equal(t, serviceStarted, true)
	equal(t, serviceClosed, false)

	// Close all factories in the container.
	equal(t, container.Close(), nil)
	equal(t, serviceClosed, true)

	<-container.Done()
}
