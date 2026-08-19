package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// The knownTxIds optimisation lets a caller replace any transaction storage
// already knows with a bare txid. go-sdk's BEEF validator does not accept a bare
// txid as an anchor, so an unproven raw transaction sitting above one cannot be
// traced to a merkle proof and the whole BEEF is invalid - which is how a
// createAction whose ancestry storage is holding itself came back as
// "provided beef is not valid". Storage must complete the ancestry it knows
// before verifying.
func TestCreateActionWithInputBEEFWhoseAncestorIsOnlyATxID(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and: a chain the caller owns - an unproven parent (known to storage) and
	// the child it is spending (unknown to storage, sent as raw bytes).
	parentSpec := txtestabilities.GivenTX().
		WithInput(200_000).
		WithP2PKHOutput(150_000)

	// the parent pays its recipient, so the child must unlock with that key
	childSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(parentSpec.TX(), 0).
		WithP2PKHOutput(100_000)

	// and: storage knows the parent, together with the ancestry it was submitted with.
	given.RecordKnownTx(parentSpec.TX(), parentSpec.BEEF().Bytes(), wdk.ProvenTxStatusUnmined)

	// and: the caller sends the parent as a bare txid, exactly as the wallet
	// does for every txid it advertised as known.
	inputBEEF := transaction.NewBeefV2()
	_, err := inputBEEF.MergeRawTx(childSpec.TX().Bytes(), nil)
	require.NoError(t, err)
	inputBEEF.MergeTxidOnly(parentSpec.ID())

	inputBEEFBytes, err := inputBEEF.Bytes()
	require.NoError(t, err)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Outputs = []wdk.ValidCreateActionOutput{}
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: childSpec.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "input whose parent was sent as a bare txid",
	}}
	args.InputBEEF = inputBEEFBytes

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)

	// and: the ancestry storage filled in travels with the action.
	resultBeef, err := transaction.NewBeefFromBytes(result.InputBeef)
	require.NoError(t, err)

	childInBeef := resultBeef.FindTransaction(childSpec.ID().String())
	require.NotNil(t, childInBeef, "the caller's raw transaction must survive")

	parentInBeef := resultBeef.FindTransaction(parentSpec.ID().String())
	require.NotNil(t, parentInBeef, "the bare txid must have been completed from storage")
	assert.NotEmpty(t, parentInBeef.Inputs, "the completed parent must carry its raw bytes, not just its txid")
}

// A bare txid for a transaction storage does NOT know must still be rejected,
// and the error must name the transaction instead of the opaque
// "provided beef is not valid".
func TestCreateActionRejectsInputBEEFWhoseAncestorIsUnknownToStorage(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and: the parent is never recorded as known.
	parentSpec := txtestabilities.GivenTX().
		WithInput(200_000).
		WithP2PKHOutput(150_000)

	// the parent pays its recipient, so the child must unlock with that key
	childSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(parentSpec.TX(), 0).
		WithP2PKHOutput(100_000)

	inputBEEF := transaction.NewBeefV2()
	_, err := inputBEEF.MergeRawTx(childSpec.TX().Bytes(), nil)
	require.NoError(t, err)
	inputBEEF.MergeTxidOnly(parentSpec.ID())

	inputBEEFBytes, err := inputBEEF.Bytes()
	require.NoError(t, err)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Outputs = []wdk.ValidCreateActionOutput{}
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: childSpec.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "input whose parent is unknown to storage",
	}}
	args.InputBEEF = inputBEEFBytes

	// when:
	_, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), parentSpec.ID().String())
}
