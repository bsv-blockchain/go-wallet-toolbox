package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletCreateActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.CreateActionArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
		"empty args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				return sdk.CreateActionArgs{}
			},
		},
		"invalid description (too short)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = "a"
				return args
			},
		},
		"invalid description (too long)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = strings.Repeat("a", 2001)
				return args
			},
		},
		"too big output satoshis": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].Satoshis = primitives.MaxSatoshis + 1
				return args
			},
		},
		"too short output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = "a"
				return args
			},
		},
		"too long output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = strings.Repeat("a", 2001)
				return args
			},
		},
	}
	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then := testabilities.New(t)

			// and:
			aliceWallet, cleanup := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)
			defer cleanup()

			// when:
			action, err := aliceWallet.CreateAction(t.Context(), test.args(), test.originator)

			// then:
			then.Result(action).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletCreateActionSuccess() {
	s.Run("return signable transaction when sign&process is false", func() {
		t := s.T()
		const topUpValue = 99904

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Options.SignAndProcess = to.Ptr(false)

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")
		require.NotNil(t, result.SignableTransaction, "Wallet result without sign&process contain signable transaction")
		assert.NotEmpty(t, result.SignableTransaction.Reference, "Signable transaction should have reference")

		// and:
		require.NotEmpty(t, result.SignableTransaction.Tx, "Signable transaction should have transaction bytes")

		thenTx := asserttx.RestoredFromBEEFBytes(t, result.SignableTransaction.Tx)

		thenTx.HasInputsThatFundsOutputs().HasMinimalFee()

		thenTx.Inputs().AllHaveUnlockingScript().HasTotalInputValue(topUpValue)

		thenTx.Outputs().AllHaveLockingScript()

		thenTx.Output(0).
			HasLockingScript(args.Outputs[0].LockingScript).
			HasSatoshis(args.Outputs[0].Satoshis).
			IsNotChange()

	})

	s.Run("return signable transaction with provided input when sign&process is false", func() {
		t := s.T()
		const topUpValue = 99904
		const inputValue = 100

		// given:
		given := testabilities.Given(t)

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.InputBEEF = input.InputBEEFBytes()
		args.Inputs = []sdk.CreateActionInput{
			input.CreateActionInput(),
		}
		args.Options.SignAndProcess = to.Ptr(false)

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")
		require.NotNil(t, result.SignableTransaction, "Wallet result without sign&process contain signable transaction")
		assert.NotEmpty(t, result.SignableTransaction.Reference, "Signable transaction should have reference")

		// and:
		require.NotEmpty(t, result.SignableTransaction.Tx, "Signable transaction should have transaction bytes")

		thenTx := asserttx.RestoredFromBEEFBytes(t, result.SignableTransaction.Tx)

		thenTx.HasInputsThatFundsOutputs().HasMinimalFee()

		thenTx.Inputs().AllHaveUnlockingScript().HasTotalInputValue(topUpValue + inputValue)

		thenTx.Outputs().AllHaveLockingScript()

		thenTx.Output(0).
			HasLockingScript(args.Outputs[0].LockingScript).
			HasSatoshis(args.Outputs[0].Satoshis).
			IsNotChange()

		// when: verify that ListActions shows the created action
		listArgs := fixtures.DefaultWalletListActionsArgsWithIncludes()
		listArgs.Labels = args.Labels
		listResult, listErr := aliceWallet.ListActions(t.Context(), listArgs, fixtures.DefaultOriginator)

		// and:
		assert.NoError(t, listErr)
		require.NotNil(t, listResult)
		assert.Equal(t, uint32(1), listResult.TotalActions, "Should have one action after create")
		require.Len(t, listResult.Actions, 1)

	})

}

func (s *WalletTestSuite) TestWalletCreateActionError() {
	s.Run("return error when user have not enough funds", func() {
		t := s.T()
		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		require.Nil(t, result)

	})

	s.Run("return error when user have not enough funds and when sign&process is false", func() {
		t := s.T()
		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Options.SignAndProcess = to.Ptr(false)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		require.Nil(t, result)

	})
}
