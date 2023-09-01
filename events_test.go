package gontainer

import (
	"reflect"
	"testing"
)

// TestEvents tests events broker.
func TestEvents(t *testing.T) {
	testEvent1Args := [][]any(nil)
	testEvent2Args := [][]any(nil)

	ev := make(events)
	ev.Subscribe("TestEvent1", func(args ...any) {
		testEvent1Args = append(testEvent1Args, args)
	})
	ev.Subscribe("TestEvent2", func(args ...any) error {
		testEvent2Args = append(testEvent2Args, args)
		return nil
	})

	equal(t, ev.Trigger(NewEvent("TestEvent1", 1)), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent1", "x")), nil)
	equal(t, ev.Trigger(NewEvent("TestEvent2", true)), nil)
	equal(t, testEvent1Args, [][]any{{1}, {"x"}})
	equal(t, testEvent2Args, [][]any{{true}})
}

func equal(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("equal failed: '%v' != '%v'", a, b)
	}
}
