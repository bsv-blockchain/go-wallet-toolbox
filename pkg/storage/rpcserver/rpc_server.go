package rpcserver

// Package rpcserver is DEPRECATED.
// The legacy JSON-RPC remoting layer has been replaced by the v2adapter
// (pkg/storage/v2adapter) which implements the canonical BRC-100 /storage/v1/*
// HTTP contract used by the TS wallet-storage and adapter conformance vectors.
// New code should use storage.NewServer (which delegates to v2adapter) and
// storage.NewClient (which now speaks V1 over authenticated HTTP).
// This package is retained only for potential internal generator use and will be removed.

import (
	"log/slog"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// RPCServer wraps a JSON-RPC server with request tracing.
type RPCServer struct {
	Handler *jsonrpc.RPCServer
	logger  *slog.Logger
}

// NewRPCHandler creates an RPCServer that registers the given handler under name.
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
