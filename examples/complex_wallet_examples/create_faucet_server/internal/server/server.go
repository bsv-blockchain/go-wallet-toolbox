package server

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

type Server struct {
	app *fiber.App
}

func New(cfg config.Config) *Server {
	app := fiber.New()

	deps := handlers.FaucetDeps{
		FaucetKeyHex: cfg.FaucetPrivateKey,
		Network:      cfg.Network,
		ServerURL:    cfg.ServerURL,
	}
	app.Post("/faucet", handlers.NewFaucetHandler(deps))
	app.Get("/address", handlers.NewGetAddressHandler(deps))
	app.Post("/topup", handlers.NewTopUpHandler(deps))

	return &Server{app: app}
}

func (s *Server) Start(addr string) error {
	return s.app.Listen(addr)
}
