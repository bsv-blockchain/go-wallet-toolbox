package handlers

import (
	"context"
	"fmt"
	"net/http"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/gofiber/fiber/v2"
)

type TopUpRequest struct {
	Outpoint string `json:"outpoint"` // Format: "txid:outputIndex"
}

type TopUpResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type TopUpDeps = methods.FaucetDeps

func NewTopUpHandler(deps TopUpDeps) fiber.Handler {
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

		// Use injected wallet if available
		w := deps.Wallet
		if w == nil {
			priv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: err.Error()})
			}
			w, err = wallet.New(deps.Network, priv, deps.Storage)
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
			}
		}

		if err := methods.TopUpInternalize(context.Background(), deps, w, op.Txid.String(), uint32(op.Index)); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(TopUpResponse{Status: "ok"})
	}
}

// walletForDeps creates a wallet instance bound to injected storage and faucet key
func walletForDeps(deps methods.FaucetDeps) (sdk.Interface, error) {
	priv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid faucet key: %w", err)
	}
	return wallet.New(deps.Network, priv, deps.Storage)
}
