package handlers

import (
	"context"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server/internal/methods"
	"github.com/gofiber/fiber/v2"
)

type FaucetRequest struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

type FaucetResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Txid    string `json:"txid,omitempty"`
	BEEFHex string `json:"beef_hex,omitempty"`
}

type FaucetDeps = methods.FaucetDeps

func NewFaucetHandler(deps FaucetDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req FaucetRequest
		if err := c.BodyParser(&req); err != nil || req.Address == "" || req.Amount == 0 {
			return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: "invalid request"})
		}

		txid, beefHex, err := methods.FundAddress(context.Background(), deps, req.Address, req.Amount)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(FaucetResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(FaucetResponse{Status: "ok", Message: "funded", Txid: txid, BEEFHex: beefHex})
	}
}
