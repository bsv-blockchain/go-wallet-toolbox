package logging

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-softwarelab/common/pkg/types"
)

const (
	ServiceKey           = "service"
	ErrorKey             = "error"
	UserIDKey            = "userId"
	ReferenceKey         = "reference"
	TxIDKey              = "txID"
	txIDsKey             = "txIDs"
	OperationKey         = "operation"
	ServiceErrorCountKey = "serviceErrorCount"
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

// Error returns a slog.Attr containing the provided error message under the "error" key.
func Error(err error) slog.Attr {
	return slog.String(ErrorKey, err.Error())
}

// Number makes easier creation slog.Attr based on any number (int, float, uint) or the custom type over the number type.
func Number[T types.Number](key string, value T) slog.Attr {
	var v slog.Value
	switch typedValue := any(value).(type) {
	case uint, uint8, uint16, uint32, uint64:
		v = slog.Uint64Value(uint64(value))
	case float32, float64:
		v = slog.Float64Value(float64(value))
	case int, int8, int16, int32, int64:
		v = slog.Int64Value(int64(value))
	case interface{ Int64() int64 }:
		// small optimization: to not reach for reflect,
		// when we have something easily convertible to int64 (like satoshis.Value)
		v = slog.Int64Value(typedValue.Int64())
	default:
		v = valueForTypeOverNumber(value)
	}

	return slog.Attr{Key: key, Value: v}
}

// valueForTypeOverNumber - when the value is a primitive type wrapped as a custom type,
// the simple switch case won't handle it, so we need to reach for reflection :(
func valueForTypeOverNumber[T types.Number](value T) slog.Value {
	v := reflect.ValueOf(value)
	//nolint:exhaustive //compiler handles other cases, thanks to generics.
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return slog.Uint64Value(uint64(value))
	case reflect.Float32, reflect.Float64:
		return slog.Float64Value(float64(value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return slog.Int64Value(int64(value))
	default:
		panic(fmt.Sprintf("unsupported type %T", value))
	}
}

// UserID creates a slog.Attr representing a user ID, accepting either an int or a pointer to an int.
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

func TxID(txID string) slog.Attr {
	return slog.String(TxIDKey, txID)
}

func Operation(op defs.Operation) slog.Attr {
	return slog.String(OperationKey, string(op))
}

func ServiceErrorCount(count int) slog.Attr {
	return slog.Int(ServiceErrorCountKey, count)
}

func TxIDs(txIDs []string) slog.Attr {
	if len(txIDs) == 0 {
		return slog.String(txIDsKey, "<none>")
	}
	return slog.Any(txIDsKey, txIDs)
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
