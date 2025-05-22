package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"
)

// SlogGormLogger implements gorm/logger.Interface using slog
type SlogGormLogger struct {
	logger *slog.Logger
}

// Info logs info-level messages
func (l *SlogGormLogger) Info(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, fmt.Sprintf(msg, args...))
}

// Warn logs warn-level messages
func (l *SlogGormLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, fmt.Sprintf(msg, args...))
}

// Error logs error-level messages
func (l *SlogGormLogger) Error(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, fmt.Sprintf(msg, args...))
}

// Trace logs SQLCommon queries with execution time
func (l *SlogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	duration := time.Since(begin)

	l.logger.DebugContext(ctx, "SQL Query",
		"sql", sql,
		"rows", rows,
		"duration", duration,
		"error", err,
	)
}

// LogMode allows changing the logging level dynamically
func (l *SlogGormLogger) LogMode(_ logger.LogLevel) logger.Interface {
	l.logger.Error("LogMode is not supported. You need to instantiate a database with new logger with the desired log level")
	return l
}
