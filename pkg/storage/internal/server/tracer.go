package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/go-softwarelab/common/pkg/to"
)

type rpcCallTracer struct {
	debug   bool
	logger  *slog.Logger
	method  string
	params  []reflect.Value
	results []reflect.Value
	err     error
}

func newTracer(logger *slog.Logger) jsonrpc.Tracer {
	return func(method string, params []reflect.Value, results []reflect.Value, err error) {
		tracer := rpcCallTracer{
			debug:   logging.IsDebug(logger),
			logger:  logger,
			method:  method,
			params:  params,
			results: results,
			err:     err,
		}

		tracer.LogRequest()

		tracer.LogResult()
	}
}

func (t *rpcCallTracer) LogRequest() {
	args := []slog.Attr{
		slog.String("method", t.method),
	}
	if t.debug {
		args = append(args, slog.String("handler", t.handlerName()))
		// params contains all the method arguments (it treats method receiver also as a param and put it in index 0)
		for i, param := range t.params[1:] {
			args = append(args, slog.Any(fmt.Sprintf("param_%d", i), t.reflectValueToLoggable(param)))
		}
	}

	t.logger.LogAttrs(t.context(), slog.LevelInfo, "RPC request", args...)
}

func (t *rpcCallTracer) LogResult() {
	err := t.checkError()
	level := to.IfThen(err != nil, slog.LevelError).ElseThen(slog.LevelInfo)

	args := []slog.Attr{
		slog.String("method", t.method),
	}

	if err != nil {
		args = append(args, logging.Error(err))
	}

	if t.debug {
		args = append(args, slog.String("handler", t.handlerName()))
		for i, result := range t.results {
			args = append(args, slog.Any(fmt.Sprintf("result_%d", i), t.reflectValueToLoggable(result)))
		}
	}

	t.logger.LogAttrs(t.context(), level, "RPC result", args...)
}

func (t *rpcCallTracer) checkError() error {
	if t.err != nil {
		return t.err
	}

	if len(t.results) == 0 {
		return nil
	}

	lastRes := t.results[len(t.results)-1]
	err, ok := lastRes.Interface().(error)
	if ok {
		return err
	}
	return nil
}

func (t *rpcCallTracer) handlerName() string {
	return t.params[0].Type().String()
}

func (t *rpcCallTracer) context() context.Context {
	// param[0] is a method receiver - which is our storage.
	// We're assuming that context will be the first argument of the method,
	// therefore we need to check if there are 2 params (1 for storage struct - method receiver, 2 for context)
	// and if the first argument is really the context.
	if len(t.params) < 2 {
		return context.Background()
	}
	ctx, ok := t.params[1].Interface().(context.Context)
	if !ok {
		return context.Background()
	}
	return ctx
}

func (t *rpcCallTracer) reflectValueToLoggable(v reflect.Value) string {
	if !v.IsValid() {
		return "<invalid>"
	}

	underlyingValue := v.Interface()

	if err, ok := underlyingValue.(error); ok {
		return fmt.Sprintf("<error: %v>", err)
	}

	if ctx, ok := underlyingValue.(context.Context); ok {
		return fmt.Sprintf("<context: %v>", ctx)
	}

	jsonBytes, err := json.Marshal(underlyingValue)
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}

	return string(jsonBytes)
}
