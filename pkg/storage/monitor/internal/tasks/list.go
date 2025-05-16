package tasks

import "github.com/4chain-ag/go-wallet-toolbox/pkg/defs"

var All = map[defs.MonitorTask]TaskCreator{
	defs.TimeMonitorTask: NewTimeTask,
}
