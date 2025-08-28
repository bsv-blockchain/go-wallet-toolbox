package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
)

type taskFactoryFunc func() tasks.TaskInterface

func (d *Daemon) allTasksFactories() map[defs.MonitorTask]taskFactoryFunc {
	return map[defs.MonitorTask]taskFactoryFunc{
		defs.CheckForProofsMonitorTask: func() tasks.TaskInterface {
			return tasks.NewCheckForProofsTask(d.storage)
		},
		defs.SendWaitingMonitorTask: func() tasks.TaskInterface {
			return tasks.NewSendWaitingTask(d.storage)
		},
		defs.FailAbandonedMonitorTask: func() tasks.TaskInterface {
			return tasks.NewFailAbandonedTask(d.storage)
		},
	}
}
