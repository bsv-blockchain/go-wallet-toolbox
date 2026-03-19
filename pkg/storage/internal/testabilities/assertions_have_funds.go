package testabilities

import (
	"fmt"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type FundsAssertion interface {
	CanReserveSatoshis(amount uint64) bool
	ShouldBeAbleToReserveSatoshis(amount uint64)
	ShouldNotBeAbleToReserveSatoshis(amount uint64)
}

func ThenFunds(t testing.TB, sender testusers.User, activeStorage wdk.WalletStorageProvider) FundsAssertion {
	return &thenFundsAssertion{
		TB:            t,
		activeStorage: activeStorage,
		sender:        sender,
	}
}

type thenFundsAssertion struct {
	testing.TB

	activeStorage wdk.WalletStorageProvider
	sender        testusers.User
}

func (t *thenFundsAssertion) CanReserveSatoshis(amount uint64) bool {
	amount -= 1 // leave 1 satoshi for fee

	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}

	lockingScript, err := brc29.LockForCounterparty(t.sender.PrivateKey(t), keyID, t.sender.PublicKey(t))
	require.NoError(t.TB, err)

	args := wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      primitives.HexString(lockingScript.String()),
				Satoshis:           primitives.SatoshiValue(amount),
				OutputDescription:  "output sent to self",
				CustomInstructions: to.Ptr(fmt.Sprintf(`{"derivationPrefix":"%s","derivationSuffix":"%s","type":"BRC29"}`, fixtures.DerivationPrefix, fixtures.DerivationSuffix)),
				Tags:               []primitives.StringUnder300{fixtures.CreateActionTestTag},
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{fixtures.CreateActionTestLabel},
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(primitives.BooleanDefaultTrue(false)),
			SendWith:               []primitives.TXIDHexString{},
			SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
			KnownTxids:             []primitives.TXIDHexString{},
			NoSendChange:           []wdk.OutPoint{},
			RandomizeOutputs:       false,
			TrustSelf:              to.Ptr(sdk.TrustSelfKnown),
		},
		IsSendWith:                   false,
		IsDelayed:                    false,
		IsNoSend:                     false,
		IsNewTx:                      true,
		IsRemixChange:                false,
		IsSignAction:                 false,
		IncludeAllSourceTransactions: false,
	}

	result, err := t.activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)
	if err != nil {
		t.Logf("CreateAction error: %v", err)
		return false
	}

	abortResult, err := t.activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(result.Reference),
	})
	require.NoError(t, err)
	require.True(t, abortResult.Aborted)

	return true
}

func (t *thenFundsAssertion) ShouldBeAbleToReserveSatoshis(amount uint64) {
	require.True(t, t.CanReserveSatoshis(amount), "should be able to reserve %d satoshis", amount)
}

func (t *thenFundsAssertion) ShouldNotBeAbleToReserveSatoshis(amount uint64) {
	require.False(t, t.CanReserveSatoshis(amount), "should NOT be able to reserve %d satoshis", amount)
}
