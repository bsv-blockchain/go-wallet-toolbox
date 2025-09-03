package handlers

import (
	"net/http"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
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

		w := deps.Wallet
		if w == nil {
			priv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(AddressResponse{Status: "error", Message: err.Error()})
			}
			w, err = wallet.New(deps.Network, priv, deps.Storage)
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(AddressResponse{Status: "error", Message: err.Error()})
			}
		}

		balance, err := methods.ComputeBalance(c.Context(), w, "default")
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
