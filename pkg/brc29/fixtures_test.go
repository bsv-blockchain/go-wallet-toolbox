package brc29_test

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

const (
	senderPrivateKeyHex    = "143ab18a84d3b25e1a13cefa90038411e5d2014590a2a4a57263d1593c8dee1c"
	senderPublicKeyHex     = "0320bbfb879bbd6761ecd2962badbb41ba9d60ca88327d78b07ae7141af6b6c810"
	senderWIFString        = "Kwu2vS6fqkd5WnRgB9VXd4vYpL9mwkXePZWtG9Nr5s6JmfHcLsQr"
	recipientPrivateKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"
	recipientPublicKeyHex  = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	invalidKeyHex          = "invalid"
	derivationPrefix       = "Pr=="
	derivationSuffix       = "Su=="
	expectedAddress        = "19bxE1pRYYtjZeQm7P8e2Ws5zMkm8NNuxx"
	expectedTestnetAddress = "mp7uX4uQMaKzLktNpx71rS5QrMMTzDP12u"
)

var keyID = brc29.KeyID{DerivationPrefix: derivationPrefix, DerivationSuffix: derivationSuffix}
