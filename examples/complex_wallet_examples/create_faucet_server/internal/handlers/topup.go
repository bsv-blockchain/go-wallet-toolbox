package handlers

import (
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/gofiber/fiber/v2"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
)

type TopUpRequest struct {
	Outpoint string `json:"outpoint"`
}

type TopUpResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func NewTopUpHandler(deps methods.FaucetDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req TopUpRequest
		if err := c.BodyParser(&req); err != nil || req.Outpoint == "" {
			return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: "outpoint required (format: txid:outputIndex)"})
		}

		// Parse outpoint using transaction helper
		op, err := transaction.OutpointFromString(req.Outpoint)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: fmt.Sprintf("invalid outpoint: %v", err)})
		}

		if err := methods.TopUpInternalize(c.Context(), deps, deps.Wallet, op.Txid.String(), op.Index); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(TopUpResponse{Status: "ok"})
	}
}
