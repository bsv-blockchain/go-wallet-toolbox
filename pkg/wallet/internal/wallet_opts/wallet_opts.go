package wallet_opts

import sdk "github.com/bsv-blockchain/go-sdk/wallet"

type Opts struct {
	Flags
}

type Flags struct {
	// IncludeAllSourceTransactions
	// If true, signableTransactions will include sourceTransaction for each input,
	// including those that do not require signature and those that were also contained in the inputBEEF.
	IncludeAllSourceTransactions bool

	// AutoKnownTxids
	// If true, txids that are known to the wallet's party beef do not need to be returned from storage.
	AutoKnownTxids bool

	// TrustSelf controls behavior of input BEEF validation.
	// If "known", input transactions may omit supporting validity proof data for all TXIDs known to this wallet.
	// If nil, input BEEFs must be complete and valid.
	TrustSelf *sdk.TrustSelf
}
