package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
	"github.com/gofiber/fiber/v2"
)

type FaucetRequest struct {
	Outputs []methods.FaucetOutput `json:"outputs"`
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
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: "invalid request format"})
		}

		if len(req.Outputs) == 0 {
			return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: "at least one output is required"})
		}

		// Validate each output
		totalAmount := uint64(0)
		for _, output := range req.Outputs {
			if output.Address == "" {
				return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: "address is required for all outputs"})
			}
			if output.Amount == 0 {
				return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: "amount must be greater than 0 for all outputs"})
			}
			totalAmount += output.Amount
		}

		// Check total amount limit
		if totalAmount > constants.MaxFaucetTotalAmount {
			return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: fmt.Sprintf("total amount must be less than %d satoshis", constants.MaxFaucetTotalAmount)})
		}

		txid, beefHex, err := methods.FundAddresses(context.Background(), deps, req.Outputs)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(FaucetResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(FaucetResponse{Status: "ok", Message: "funded", Txid: txid, BEEFHex: beefHex})
	}
}
