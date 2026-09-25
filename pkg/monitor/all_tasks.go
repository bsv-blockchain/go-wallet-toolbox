package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type taskFactoryFunc func() tasks.TaskInterface

// publisherOrNil keeps a nil queue a nil interface, so tasks can tell that
// nobody subscribed and skip building events.
func publisherOrNil(q *eventqueue.Queue[wdk.CurrentTxStatus]) tasks.StatusPublisher {
	if q == nil {
		return nil
	}
	return q
}

func (d *Daemon) allTasksFactories() map[defs.MonitorTask]taskFactoryFunc {
	return map[defs.MonitorTask]taskFactoryFunc{
		defs.CheckForProofsMonitorTask: func() tasks.TaskInterface {
			return tasks.NewCheckForProofsTask(d.storage, publisherOrNil(d.txProvenEvents))
		},
		defs.SendWaitingMonitorTask: func() tasks.TaskInterface {
			return tasks.NewSendWaitingTask(d.storage, publisherOrNil(d.txBroadcastedEvents))
		},
		defs.FailAbandonedMonitorTask: func() tasks.TaskInterface {
			return tasks.NewFailAbandonedTask(d.storage)
		},
		defs.UnFailMonitorTask: func() tasks.TaskInterface {
			return tasks.NewUnFailTask(d.storage)
		},
	}
}
