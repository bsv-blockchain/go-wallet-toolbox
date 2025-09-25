package server

import (
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/filecoin-project/go-jsonrpc"
)

type RPCServer struct {
	Handler *jsonrpc.RPCServer
	logger  *slog.Logger
}

func NewRPCHandler(parentLogger *slog.Logger, name string, handler any) *RPCServer {
	logger := logging.Child(parentLogger, "RPCServer")

	rpcServer := jsonrpc.NewServer(
		jsonrpc.WithServerMethodNameFormatter(jsonrpc.NewMethodNameFormatter(false, jsonrpc.LowerFirstCharCase)),
		jsonrpc.WithTracer(newTracer(logger)),
	)

	rpcServer.Register(name, handler)

	return &RPCServer{
		Handler: rpcServer,
		logger:  logger,
	}
}

func (s *RPCServer) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /{$}", s.Handler.ServeHTTP)
}
