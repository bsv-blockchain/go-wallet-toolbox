package logging

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

const (
	ServiceKey   = "service"
	ErrorKey     = "error"
	UserIDKey    = "userId"
	ReferenceKey = "reference"
)

var strLevelToSlog = map[defs.LogLevel]slog.Level{
	defs.LogLevelDebug: slog.LevelDebug,
	defs.LogLevelInfo:  slog.LevelInfo,
	defs.LogLevelWarn:  slog.LevelWarn,
	defs.LogLevelError: slog.LevelError,
}

// Child returns a new logger with the given service name added to the logger attrs.
func Child(logger *slog.Logger, serviceName string) *slog.Logger {
	return DefaultIfNil(logger).With(
		slog.String(ServiceKey, serviceName),
	)
}

func Error(err error) slog.Attr {
	return slog.String(ErrorKey, err.Error())
}

func UserID[ID int | *int](userID ID) slog.Attr {
	switch id := any(userID).(type) {
	case int:
		return slog.Int(UserIDKey, id)
	case *int:
		if id == nil {
			return slog.String(UserIDKey, "<unknown>")
		}
		return slog.Int(UserIDKey, *id)
	default:
		panic(fmt.Sprintf("unsupported type %T", id))
	}
}

func Reference(ref string) slog.Attr {
	return slog.String(ReferenceKey, ref)
}

// DefaultIfNil returns the default logger if the given logger is nil.
func DefaultIfNil(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func IsDebug(logger *slog.Logger) bool {
	return logger.Enabled(context.Background(), slog.LevelDebug)
}
