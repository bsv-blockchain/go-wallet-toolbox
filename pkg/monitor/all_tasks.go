package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
)

type taskFactoryFunc func() tasks.TaskInterface

func (d *Daemon) allTasksFactories() map[defs.MonitorTask]taskFactoryFunc {
	return map[defs.MonitorTask]taskFactoryFunc{
		defs.CheckForProofsMonitorTask: func() tasks.TaskInterface {
			return tasks.NewCheckForProofsTask(d.storage, d.eventChannels.OnTxProven, d.logger)
		},
		defs.SendWaitingMonitorTask: func() tasks.TaskInterface {
			return tasks.NewSendWaitingTask(d.storage, d.eventChannels.OnTxBroadcasted, d.logger)
		},
		defs.FailAbandonedMonitorTask: func() tasks.TaskInterface {
			return tasks.NewFailAbandonedTask(d.storage)
		},
		defs.UnFailMonitorTask: func() tasks.TaskInterface {
			return tasks.NewUnFailTask(d.storage)
		},
	}
}
