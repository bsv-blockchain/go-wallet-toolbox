package wallet_opts

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	wallet_settings_manager "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_settings_manager"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

type Opts struct {
	Flags
	Services               *services.WalletServices
	Logger                 *slog.Logger
	PendingSignActionsRepo pending.SignActionsRepository
	Client                 *http.Client
	WalletSettingsManager  *wallet_settings_manager.WalletSettingsManager
	LookupResolver         *lookup.LookupResolver
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

func DefaultClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // connection timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive
		}).DialContext,
		ForceAttemptHTTP2:     true,             // enable HTTP/2 if supported
		MaxIdleConns:          100,              // total idle connections
		MaxIdleConnsPerHost:   10,               // idle connections per host
		IdleConnTimeout:       90 * time.Second, // keep idle connections alive
		TLSHandshakeTimeout:   5 * time.Second,  // TLS handshake timeout
		ExpectContinueTimeout: 1 * time.Second,  // for requests with Expect: 100-continue
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second, // overall request timeout
	}
}
