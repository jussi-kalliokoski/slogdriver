package slogdriver

import (
	"context"
	"testing"
)

func TestLabels(t *testing.T) {
	t.Run("nested labels", func(t *testing.T) {
		ctx := context.Background()
		l1 := NewLabel("key1", "value1")
		l2 := NewLabel("key2", "value2")
		l3 := NewLabel("key3", "value3")
		l4 := NewLabel("key4", "value4")
		ctx = AddLabels(ctx, l1, l2)
		ctx = AddLabels(ctx, l3, l4)
		expected := []Label{l1, l2, l3, l4}
		received := make([]Label, 0, len(expected))

		labelsFromContext(ctx).Iterate(func(l Label) {
			received = append(received, l)
		})

		requireEqualSlices(t, expected, received)
	})
}

func requireEqualSlices[T comparable](tb testing.TB, expected, received []T) {
	if len(expected) != len(received) {
		tb.Fatalf("expected a slice of len() %d, got a slice of len() %d", len(expected), len(received))
	}
	for i := range expected {
		if expected[i] != received[i] {
			tb.Fatalf("expected a slice with value %#v at index %d, got %#v", expected[i], i, received[i])
		}
	}
}
