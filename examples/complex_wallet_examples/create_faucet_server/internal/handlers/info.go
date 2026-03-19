package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
)

type AddressResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
	Network string `json:"network"`
}

// NewGetAddressHandler returns faucet address and its current balance.
func NewGetAddressHandler(deps methods.FaucetDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		addr, err := methods.DeriveAddress(deps.FaucetPrivateKey, deps.Network)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(AddressResponse{Status: "error", Message: err.Error()})
		}

		balance, err := methods.ComputeBalance(c.Context(), deps.Wallet, "default")
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
