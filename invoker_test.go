package gontainer

import (
	"testing"
	"time"
)

// TestInvokerService tests invoker service.
func TestInvokerService(t *testing.T) {
	factoryStarted := false

	container, err := New(
		NewFactory(func() string { return "string" }),
		NewFactory(func(invoker Invoker) {
			factoryStarted = true
			invokeCalled := false
			results, err := invoker.Invoke(func(value string) int {
				equal(t, value, "string")
				invokeCalled = true
				return 123
			})

			equal(t, err, nil)
			equal(t, invokeCalled, true)
			equal(t, len(results), 1)
			equal(t, results[0], 123)
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Start all factories in the container.
	equal(t, container.Start(), nil)
	equal(t, factoryStarted, true)

	// Let async service function launch.
	time.Sleep(time.Millisecond)

	// Close all factories in the container.
	equal(t, container.Close(), nil)

	<-container.Done()
}
