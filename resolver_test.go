package gontainer

import (
	"testing"
	"time"
)

// TestResolverService tests resolver service.
func TestResolverService(t *testing.T) {
	container, err := New(
		NewFactory(func() string { return "string" }),
		NewFactory(func(resolver Resolver) {
			var depExists string
			equal(t, resolver.Resolve(&depExists), nil)
			equal(t, depExists, "string")

			var depNotExists int
			equal(t, resolver.Resolve(&depNotExists) != nil, true)
			equal(t, depNotExists, 0)
		}),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)

	// Start all factories in the container.
	equal(t, container.Start(), nil)

	// Let async service function launch.
	time.Sleep(time.Millisecond)

	// Close all factories in the container.
	equal(t, container.Close(), nil)

	<-container.Done()
}
