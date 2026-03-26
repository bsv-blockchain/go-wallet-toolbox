package logging

import (
	"fmt"
	"log/slog"
)

// RestyLogger adapts slog to the resty logging interface.
type RestyLogger interface {
	Errorf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

type restyAdapter slog.Logger

// RestyAdapter wraps an slog.Logger to satisfy the RestyLogger interface.
func RestyAdapter(logger *slog.Logger) RestyLogger {
	logger = Child(logger, "resty")

	return (*restyAdapter)(logger)
}

func (l *restyAdapter) Errorf(message string, v ...interface{}) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}
	(*slog.Logger)(l).Error(message)
}

func (l *restyAdapter) Warnf(message string, v ...interface{}) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}
	(*slog.Logger)(l).Warn(message)
}

func (l *restyAdapter) Debugf(message string, v ...interface{}) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}
	(*slog.Logger)(l).Debug(message)
}
