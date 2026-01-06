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

// Register registers the RPC handler to accept POST requests.
// Uses "POST /" pattern to allow the server to be mounted at any path
// (e.g., /wallet, /api/storage, etc.) when embedded in another application.
// The JSON-RPC protocol is path-agnostic - it only processes the request body.
func (s *RPCServer) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /", s.Handler.ServeHTTP)
}
