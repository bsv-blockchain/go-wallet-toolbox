package brc29

import sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"

type lockOptions struct {
	mainNet bool
}

// WithMainNet configures the template to use the mainnet address.
func WithMainNet() func(*lockOptions) {
	return func(o *lockOptions) {
		o.mainNet = true
	}
}

// WithTestNet configures the template to use the testnet address.
func WithTestNet() func(*lockOptions) {
	return func(o *lockOptions) {
		o.mainNet = false
	}
}

type unlockOptions struct {
	sigHash *sighash.Flag
}

// WithSigHash configures the template to use the specified sighash.
func WithSigHash(sigHash *sighash.Flag) func(*unlockOptions) {
	return func(o *unlockOptions) {
		o.sigHash = sigHash
	}
}
