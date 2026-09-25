package services

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/stretchr/testify/require"
)

func TestReorgBroadcaster_BroadcastNilEventToFullSubscriberDoesNotPanic(t *testing.T) {
	// given:
	broadcaster := newReorgBroadcaster(slog.Default())
	ch := make(chan *chaintracks.ReorgEvent)
	unsub := broadcaster.Subscribe(ch)
	defer unsub()

	// then:
	require.NotPanics(t, func() {
		broadcaster.broadcast(nil)
	})
}

func TestReorgBroadcaster_BroadcastEventToFullSubscriberDoesNotPanic(t *testing.T) {
	// given:
	broadcaster := newReorgBroadcaster(slog.Default())
	ch := make(chan *chaintracks.ReorgEvent)
	unsub := broadcaster.Subscribe(ch)
	defer unsub()

	// then:
	require.NotPanics(t, func() {
		broadcaster.broadcast(&chaintracks.ReorgEvent{})
	})
}

func TestReorgBroadcaster_SlowSubscriberMissesNoReorgs(t *testing.T) {
	// given: a subscriber with room for one event that does not read yet
	const count = 50
	broadcaster := newReorgBroadcaster(slog.Default())
	ch := make(chan *chaintracks.ReorgEvent, 1)
	unsub := broadcaster.Subscribe(ch)

	// when: more reorgs arrive than the channel can hold; broadcast must not block
	for depth := range uint32(count) {
		broadcaster.broadcast(&chaintracks.ReorgEvent{Depth: depth})
	}

	// then: every reorg is delivered, in order
	for want := range uint32(count) {
		select {
		case ev := <-ch:
			require.Equal(t, want, ev.Depth)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for reorg %d", want)
		}
	}

	// and: after unsubscribing nothing sends, so closing the channel is safe
	unsub()
	require.NotPanics(t, func() { close(ch) })
}

func TestReorgBroadcaster_UnsubscribeWithPendingEventsAllowsClose(t *testing.T) {
	// given: a subscriber that never reads, with a backlog
	broadcaster := newReorgBroadcaster(slog.Default())
	ch := make(chan *chaintracks.ReorgEvent)
	unsub := broadcaster.Subscribe(ch)
	for range 10 {
		broadcaster.broadcast(&chaintracks.ReorgEvent{})
	}

	// when:
	unsub()
	unsub() // idempotent

	// then:
	require.NotPanics(t, func() { close(ch) })
	require.NotPanics(t, func() { broadcaster.broadcast(&chaintracks.ReorgEvent{}) })
}
