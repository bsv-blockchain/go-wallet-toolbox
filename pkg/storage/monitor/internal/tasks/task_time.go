package tasks

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"log/slog"
	"time"
)

type TimeTask struct {
	logger *slog.Logger
}

func NewTimeTask(logger *slog.Logger) TaskInterface {
	return &TimeTask{
		logger: logging.Child(logger, "time_task"),
	}
}

func (t *TimeTask) Run() {
	now := time.Now()
	if now.Second() == 0 {
		t.logger.Info("current time", "time", now.Format("2006-01-02 15:04"))
	}
}
