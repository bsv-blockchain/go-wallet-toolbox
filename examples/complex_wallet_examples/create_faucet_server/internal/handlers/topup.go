package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
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

		// Parse outpoint into txid and output index
		parts := strings.Split(req.Outpoint, ":")
		if len(parts) != 2 {
			return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: "invalid outpoint format, expected txid:outputIndex"})
		}

		txid := parts[0]
		outputIndexStr := parts[1]
		outputIndex, err := strconv.ParseUint(outputIndexStr, 10, 32)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: fmt.Sprintf("invalid output index: %s", outputIndexStr)})
		}

		// Prepare wallet and identity for internalization
		ctx := context.Background()
		priv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}
		identity := priv.PubKey()

		storageClient, cleanup, err := storage.NewClient(deps.ServerURL)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}
		defer cleanup()

		w, err := wallet.New(deps.Network, priv, storageClient)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}

		if err := methods.TopUpInternalize(ctx, deps, identity, w, txid, uint32(outputIndex)); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(TopUpResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(TopUpResponse{Status: "ok"})
	}
}
