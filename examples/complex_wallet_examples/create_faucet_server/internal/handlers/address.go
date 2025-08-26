package handlers

import (
	"context"
	"net/http"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/gofiber/fiber/v2"
)

type AddressResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
	Network string `json:"network"`
}

type AddressDeps = methods.FaucetDeps

// NewGetAddressHandler returns faucet address and its current balance.
func NewGetAddressHandler(deps AddressDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		addr, err := methods.DeriveAddress(deps.FaucetKeyHex, deps.Network)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}

		ctx := context.Background()
		priv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}

		storageClient, cleanup, err := storage.NewClient(deps.ServerURL)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}
		defer cleanup()

		w, err := wallet.New(deps.Network, priv, storageClient)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}

		balance, err := methods.ComputeBalance(ctx, w, "default")
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}

		return c.JSON(AddressResponse{
			Status:  "ok",
			Address: addr,
			Balance: balance,
			Network: string(deps.Network),
		})
	}
}
