package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/certifier-server-example/internal/config"
	"github.com/bsv-blockchain/certifier-server-example/internal/example_setup"
	"github.com/bsv-blockchain/certifier-server-example/internal/service"
	httpTransport "github.com/bsv-blockchain/certifier-server-example/internal/transport/http"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Server struct {
	config *config.Config
	logger *slog.Logger
	wallet certificates.CertifierWallet
}

func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	server := &Server{
		config: cfg,
		logger: logger,
	}

	if err := server.initializeCertifierWallet(); err != nil {
		return nil, fmt.Errorf("failed to initialize wallet: %w", err)
	}

	return server, nil
}

func (s *Server) initializeCertifierWallet() error {
	privateKey, err := ec.PrivateKeyFromHex(s.config.CertifierWallet.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid certifier private key: %w", err)
	}

	identityKey := privateKey.PubKey()
	if identityKey.ToDERHex() != s.config.CertifierWallet.IdentityKey {
		return fmt.Errorf("identity key does not match the public key derived from private key")
	}

	s.wallet, err = wallet.NewWithStorageFactory(
		defs.BSVNetwork(s.config.Server.Network),
		s.config.CertifierWallet.PrivateKey,
		func(userWallet sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
			if s.config.Storage.URL != "" {
				return storage.NewClient(s.config.Storage.URL, userWallet)
			}
			return example_setup.CreateLocalStorage(
				context.Background(),
				defs.BSVNetwork(s.config.Server.Network),
				s.config.Storage.PrivateKey,
			)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create wallet: %w", err)
	}

	return nil
}

func (s *Server) setupRoutes() http.Handler {
	certificateService := service.NewCertificateService(s.wallet, s.logger)
	certificateHandler := httpTransport.NewCertificateHandler(certificateService, s.config, s.logger)

	mux := http.NewServeMux()
	mux.Handle("/", certificateHandler)

	return mux
}

func (s *Server) Start() error {
	handler := s.setupRoutes()
	addr := ":" + s.config.Server.Port

	s.logger.Info("Starting certificate server", "addr", addr, "network", s.config.Server.Network)

	return http.ListenAndServe(addr, handler) //nolint:gosec // example server, timeouts not required
}
