package server

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/gofiber/fiber/v2"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/handlers"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Server struct {
	app *fiber.App
}

func New(cfg config.Config, storage wdk.WalletStorageProvider) *Server {
	app := fiber.New()

	priv, err := ec.PrivateKeyFromHex(cfg.FaucetPrivateKey)
	if err != nil {
		panic(fmt.Errorf("invalid faucet private key: %w", err))
	}

	w, err := wallet.New(cfg.Network, priv, storage)
	if err != nil {
		panic(fmt.Errorf("failed to create faucet wallet: %w", err))
	}

	deps := methods.FaucetDeps{
		FaucetPrivateKey:     priv,
		Network:              cfg.Network,
		Storage:              storage,
		MaxFaucetTotalAmount: cfg.MaxFaucetTotalAmount,
		Wallet:               w,
	}

	app.Post("/faucet", handlers.NewFaucetHandler(deps))
	app.Get("/info", handlers.NewGetAddressHandler(deps))
	app.Post("/topup", handlers.NewTopUpHandler(deps))

	return &Server{app: app}
}

func (s *Server) Start(addr string) error {
	return s.app.Listen(addr)
}
