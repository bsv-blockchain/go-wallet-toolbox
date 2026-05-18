package services

import (
	"log/slog"
	"testing"

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
