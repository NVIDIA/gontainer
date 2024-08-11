package gontainer

import (
	"testing"
)

// TestResolverResolve tests resolver service.
func TestResolverResolve(t *testing.T) {
	container, err := New(NewFactory(func() string { return "string" }))
	equal(t, err, nil)
	equal(t, container == nil, false)
	resolver := container.Resolver()

	// Start all factories in the container.
	equal(t, container.Start(), nil)

	var depExists string
	equal(t, resolver.Resolve(&depExists), nil)
	equal(t, depExists, "string")

	var depNotExists int
	equal(t, resolver.Resolve(&depNotExists) != nil, true)
	equal(t, depNotExists, 0)

	// Close all factories in the container.
	equal(t, container.Close(), nil)
	<-container.Done()
}

// TestResolverImplements tests resolver service.
func TestResolverImplements(t *testing.T) {
	svc1 := &testService1{}
	svc2 := &testService2{}
	svc3 := &testService3{}

	container, err := New(
		NewFactory(func() *testService1 { return svc1 }),
		NewFactory(func() *testService2 { return svc2 }),
		NewFactory(func() *testService3 { return svc3 }),
	)
	equal(t, err, nil)
	equal(t, container == nil, false)
	resolver := container.Resolver()

	// Start all factories in the container.
	equal(t, container.Start(), nil)

	// Resolve all services implementing interface.
	var implements []interface{ Do2() }
	equal(t, resolver.Implements(&implements), nil)
	equal(t, len(implements), 2)
	equal(t, implements[0], svc1)
	equal(t, implements[1], svc2)

	// Close all factories in the container.
	equal(t, container.Close(), nil)
	<-container.Done()
}

type testService1 struct{}

func (t *testService1) Do1() {}
func (t *testService1) Do2() {}
func (t *testService1) Do3() {}

type testService2 struct{}

func (t *testService2) Do1() {}
func (t *testService2) Do2() {}

type testService3 struct{}

func (t *testService3) Do1() {}
