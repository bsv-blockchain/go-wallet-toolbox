package errors_test

import (
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestTransactionError_Success(t *testing.T) {
	rootCause := errors.New("root cause")
	err := pkgerrors.NewTransactionError(chainhash.Hash{}).Wrap(rootCause)
	require.NotNil(t, err)
	assert.Equal(t, "transaction error (txID: 0000000000000000000000000000000000000000000000000000000000000000)", err.Error())
	require.EqualError(t, err.Unwrap(), "root cause")
	assert.True(t, err.Is(err))
	assert.ErrorIs(t, err, rootCause)
}

func TestTransactionError_ErrorCases(t *testing.T) {
	t.Run("error without cause", func(t *testing.T) {
		err := pkgerrors.NewTransactionError(chainhash.Hash{})
		require.NotNil(t, err)
		require.NoError(t, err.Unwrap())
		assert.False(t, err.Is(nil))
	})
}

func TestCreateActionError_Success(t *testing.T) {
	err := pkgerrors.NewCreateActionError("ref1").Wrap(errors.New("build failure"))

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "create action failed (reference: ref1)")
	require.EqualError(t, err.Unwrap(), "build failure")
	assert.True(t, err.Is(err))
	assert.False(t, err.Is(nil))
}

func TestCreateActionError_ErrorCases(t *testing.T) {
	t.Run("error without cause", func(t *testing.T) {
		err := pkgerrors.NewCreateActionError("ref2")
		require.NotNil(t, err)
		require.NoError(t, err.Unwrap())
		assert.False(t, err.Is(nil))
	})
}

func TestProcessActionError_Success(t *testing.T) {
	tests := map[string]struct {
		sendResults   []wdk.SendWithResult
		reviewResults []wdk.ReviewActionResult
		cause         error
		expectedParts []string
	}{
		"with success and failed txs": {
			sendResults: []wdk.SendWithResult{
				{Status: wdk.SendWithResultStatusUnproven},
				{Status: wdk.SendWithResultStatusFailed},
				{Status: wdk.SendWithResultStatusSending},
			},
			expectedParts: []string{"3 total", "1 succeeded", "1 sending", "1 failed"},
		},
		"with review results and cause": {
			reviewResults: []wdk.ReviewActionResult{{}, {}},
			cause:         errors.New("root processing issue"),
			expectedParts: []string{"review results: 2 require review", "underlying error: root processing issue"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := pkgerrors.NewProcessActionError(test.sendResults, test.reviewResults).Wrap(test.cause)
			require.NotNil(t, err)

			message := err.Error()
			assert.Contains(t, message, "process action failed")

			for _, part := range test.expectedParts {
				assert.Contains(t, message, part)
			}

			assert.Equal(t, test.cause, err.Unwrap())
			assert.True(t, err.Is(err))
			if test.cause != nil {
				assert.ErrorIs(t, err, test.cause)
			}
		})
	}
}

func TestProcessActionError_ErrorCases(t *testing.T) {
	t.Run("nil cause and no results", func(t *testing.T) {
		err := pkgerrors.NewProcessActionError(nil, nil)
		assert.NotNil(t, err)
		assert.Equal(t, "process action failed", err.Error())
		assert.False(t, err.Is(nil))
		assert.NoError(t, err.Unwrap())
	})
}

func TestProcessActionError_WithTxAndNoSendChange(t *testing.T) {
	txID := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	txBytes := []byte{0x01, 0x00, 0xbe, 0xef}
	noSendChange := []string{txID + ".0", txID + ".1"}
	sendResults := []wdk.SendWithResult{
		{TxID: "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899", Status: wdk.SendWithResultStatusFailed},
	}
	reviewResults := []wdk.ReviewActionResult{
		{Status: wdk.ReviewActionResultStatusDoubleSpend},
	}
	cause := errors.New("unexpected not delayed results when send with results are not all unproven")

	err := pkgerrors.NewProcessActionError(sendResults, reviewResults).
		WithTx(txID, txBytes).
		WithNoSendChange(noSendChange).
		Wrap(cause)

	require.NotNil(t, err)
	assert.Equal(t, txID, err.TxID)
	assert.Equal(t, txBytes, err.Tx)
	assert.Equal(t, noSendChange, err.NoSendChange)
	assert.Equal(t, sendResults, err.SendWithResults)
	assert.Equal(t, reviewResults, err.ReviewResults)
	assert.Contains(t, err.Error(), "txID: "+txID)
	assert.Contains(t, err.Error(), "review results: 1 require review")
	assert.ErrorIs(t, err, cause)
}

func TestProcessActionError_RecoverViaErrorsAsThroughTransactionError(t *testing.T) {
	txIDHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	txBytes := []byte{0xde, 0xad, 0xbe, 0xef}
	noSendChange := []string{txIDHex + ".2"}

	processErr := pkgerrors.NewProcessActionError(
		[]wdk.SendWithResult{{Status: wdk.SendWithResultStatusFailed}},
		[]wdk.ReviewActionResult{{Status: wdk.ReviewActionResultStatusServiceError}},
	).WithTx(txIDHex, txBytes).WithNoSendChange(noSendChange).Wrap(errors.New("review required"))

	// Wallet createAction wraps ProcessActionError in TransactionError; callers must still recover fields.
	wrapped := pkgerrors.NewTransactionError(chainhash.Hash{}).Wrap(processErr)

	var recovered *pkgerrors.ProcessActionError
	require.ErrorAs(t, wrapped, &recovered)
	assert.Equal(t, txIDHex, recovered.TxID)
	assert.Equal(t, txBytes, recovered.Tx)
	assert.Equal(t, noSendChange, recovered.NoSendChange)
	assert.Len(t, recovered.SendWithResults, 1)
	assert.Len(t, recovered.ReviewResults, 1)

	// Existing error interface behavior preserved.
	var txErr *pkgerrors.TransactionError
	require.ErrorAs(t, wrapped, &txErr)
	assert.True(t, errors.Is(wrapped, processErr))
}
