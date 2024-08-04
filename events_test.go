package gontainer

import (
	"reflect"
	"testing"
)

// TestEvents tests events broker.
func TestEvents(t *testing.T) {
	testEvent1Args := [][]any(nil)
	testEvent2Args := [][]any(nil)
	testEvent3Args := [][]any(nil)

	ev := &events{events: make(map[string][]handler)}
	ev.Subscribe("TestEvent1", func(args ...any) {
		testEvent1Args = append(testEvent1Args, args)
	})
	ev.Subscribe("TestEvent2", func(args ...any) error {
		testEvent2Args = append(testEvent2Args, args)
		return nil
	})
	ev.Subscribe("TestEvent3", func(x string, y int, z bool) error {
		testEvent3Args = append(testEvent3Args, []any{x, y, z})
		return nil
	})

	equal(t, ev.Trigger(NewEvent("TestEvent1", 1)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent1", "x")), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent2", true)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent3", "x", 1, true)), nil)
	equal(t, testEvent1Args, [][]any{{1}, {"x"}})
	equal(t, testEvent2Args, [][]any{{true}})
	equal(t, testEvent3Args, [][]any{{"x", 1, true}})
}

func equal(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("equal failed: '%v' != '%v'", a, b)
	}
}
