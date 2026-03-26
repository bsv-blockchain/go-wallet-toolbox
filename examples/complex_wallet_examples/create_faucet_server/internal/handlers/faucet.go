package handlers

import (
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
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

func NewFaucetHandler(deps methods.FaucetDeps) fiber.Handler {
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
		for i, output := range req.Outputs {
			if output.Address == "" {
				return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: fmt.Sprintf("address is missing in output %d", i)})
			}
			if output.Amount == 0 {
				return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: fmt.Sprintf("amount equal to zero is not allowed in output %d", i)})
			}
			totalAmount += output.Amount
		}

		if deps.MaxFaucetTotalAmount > 0 && totalAmount > deps.MaxFaucetTotalAmount {
			return c.Status(http.StatusBadRequest).JSON(FaucetResponse{Status: "error", Message: fmt.Sprintf("total amount must be <= %d satoshis", deps.MaxFaucetTotalAmount)})
		}

		txid, beefHex, err := methods.FundAddress(c.Context(), deps, req.Outputs...)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(FaucetResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(FaucetResponse{Status: "ok", Message: "funded", Txid: txid, BEEFHex: beefHex})
	}
}
