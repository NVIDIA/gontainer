package gontainer

import (
	"context"
	"fmt"
	"testing"
)

// TestFactoryLoad tests factory loading.
func TestFactoryLoad(t *testing.T) {
	fun := func(a, b, c string) (int, bool, error) {
		return 1, true, nil
	}

	ctx := context.Background()
	opts := WithSubscribe("test", func() {})
	factory := NewFactory(fun, opts)

	equal(t, factory.load(ctx), nil)
	equal(t, factory.factoryFunc == nil, false)
	equal(t, factory.factoryLoaded, true)
	equal(t, factory.factorySpawned, false)
	equal(t, factory.factoryCtx != ctx, true)
	equal(t, factory.factoryCtx != nil, true)
	equal(t, factory.ctxCancel != nil, true)
	equal(t, factory.factoryType.String(), "func(string, string, string) (int, bool, error)")
	equal(t, factory.factoryValue.String(), "<func(string, string, string) (int, bool, error) Value>")
	equal(t, fmt.Sprint(factory.factoryInTypes), "[string string string]")
	equal(t, fmt.Sprint(factory.factoryOutTypes), "[int bool]")
	equal(t, factory.factoryOutError, true)
	equal(t, len(factory.factoryOutValues), 0)
	equal(t, len(factory.factoryEvents["test"]), 1)
	equal(t, fmt.Sprint(factory.factoryEventsTypes), "map[test:[func()]]")
	equal(t, fmt.Sprint(factory.factoryEventsValues), "map[test:[<func() Value>]]")
	equal(t, fmt.Sprint(factory.factoryEventsInTypes), "map[func():[]]")
	equal(t, fmt.Sprint(factory.factoryEventsOutErrors), "map[func():false]")
}
