package server

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/filecoin-project/go-jsonrpc"
)

type RPCServer struct {
	Handler *jsonrpc.RPCServer
	logger  *slog.Logger
}

func NewRPCHandler(parentLogger *slog.Logger, name string, handler any) *RPCServer {
	logger := logging.Child(parentLogger, "rpc_server")

	rpcServer := jsonrpc.NewServer(
		jsonrpc.WithServerMethodNameFormatter(jsonrpc.NewMethodNameFormatter(false, jsonrpc.LowerFirstCharCase)),
		jsonrpc.WithTracer(tracer(logger)),
	)

	rpcServer.Register(name, handler)

	return &RPCServer{
		Handler: rpcServer,
		logger:  logger,
	}
}

func (s *RPCServer) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /{$}", s.Handler.ServeHTTP)
	mux.HandleFunc("POST /.well-known/auth", s.handleAuth) // fixme: this is a workaround to pass the client to the next step, it will be handled by the auth middleware
}

func (s *RPCServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	s.logger.Warn("Auth requests are still not handled properly, this is a workaround to pass the client to the next step, it will be handled by the auth middleware")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Warn("Couldn't read body from auth request")
	}
	err = r.Body.Close()
	if err != nil {
		s.logger.Warn("Couldn't close body from auth request")
	}

	headers := &strings.Builder{}

	err = r.Header.Write(headers)
	if err != nil {
		s.logger.Warn("Couldn't write headers from auth request")
	}

	s.logger.Debug("Received auth request",
		slog.String("body", string(body)),
		slog.String("headers", headers.String()),
		slog.String("path", r.URL.String()),
	)

	// from-kt: this is a workaround to pass the client to the next step
	w.WriteHeader(http.StatusInternalServerError)
}
