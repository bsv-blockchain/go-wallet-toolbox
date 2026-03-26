package testservices

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/spv"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
)

const (
	DeploymentID  = "go-wallet-toolbox-test"
	TestBlockHash = "0000000014209ae688e547a58db514ac75e3a10a81ac25b3d357fa92a8ce5128"
)

const (
	arcHttpStatusMalformed                     = 463
	arcHttpStatusCumulativeFeeValidationFailed = 473
)

var timestamp = time.Date(2018, time.November, 10, 23, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.999999999Z")

type ARCFixture interface {
	IsUpAndRunning()
	HttpClient() *resty.Client
	TxInfoJSON(id string) string
	WillAlwaysReturnStatus(httpStatus int)
	WhenQueryingTx(txID string) ARCQueryFixture
	OnBroadcast() ArcBroadcastFixture
	HoldBroadcasting() ARCFixture
	ReleaseBroadcasting() ARCFixture
}

type ARCQueryFixture interface {
	WillReturnHttpStatus(httpStatus int)
	WillBeUnreachable()
	WillReturnNoBody()
	WillReturnDifferentTxID()
	WillReturnDoubleSpending(competingTxs ...string)
	WillReturnTransactionWithoutMerklePath()
	WillReturnTransactionWithMerklePathHex(merklePath string)
	WillReturnTransactionWithMerklePath(path sdk.MerklePath) ARCQueryFixture
	WillReturnWithMindedTx() ARCQueryFixture
	WillReturnTransactionOnHeight(i int)
	WillReturnTransactionWithBlockHash(hash *chainhash.Hash)
	WillReturnTransactionWithBlockHeight(height uint32)
}

type ArcBroadcastFixture interface {
	WillReturnNoBody()
}

type arcFixture struct {
	testing.TB

	transport                    *httpmock.MockTransport
	knownTransactions            sync.Map
	broadcastWithoutResponseBody bool
	network                      defs.BSVNetwork
	url                          string
	token                        string
	holdBroadcastExecution       sync.RWMutex
}

func NewARCFixture(t testing.TB, opts ...Option) ARCFixture {
	options := to.OptionsWithDefault(FixtureOptions{
		network:   defs.NetworkMainnet,
		transport: httpmock.NewMockTransport(),
	}, opts...)

	return &arcFixture{
		TB:        t,
		transport: options.transport,
		network:   options.network,
		url:       to.IfThen(options.network == defs.NetworkMainnet, defs.ArcURL).ElseThen(defs.ArcTestURL),
		token:     to.IfThen(options.network == defs.NetworkMainnet, defs.ArcToken).ElseThen(defs.ArcTestToken),
	}
}

func (f *arcFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.SetTransport(f.transport)
	return client
}

func (f *arcFixture) WillAlwaysReturnStatus(httpStatus int) {
	f.transport.RegisterResponder("POST", "=~"+f.url+"/v1/tx.*", func(req *http.Request) (*http.Response, error) {
		return httpmock.NewJsonResponse(errorResponseForStatus(httpStatus))
	})
}

func (f *arcFixture) IsUpAndRunning() {
	f.transport.RegisterResponder(http.MethodPost, f.url+"/v1/tx", func(req *http.Request) (*http.Response, error) {
		f.holdBroadcastExecution.RLock()
		defer f.holdBroadcastExecution.RUnlock()

		b, err := io.ReadAll(req.Body)
		require.NoError(f, err)

		var body map[string]any
		err = json.Unmarshal(b, &body)
		require.NoError(f, err)

		rawTx := body["rawTx"]
		if !assert.NotNil(f, rawTx) {
			return httpmock.NewJsonResponse(
				errorResponseForStatusWithExtraInfo(
					http.StatusBadRequest,
					"error parsing transactions from request: no transaction found - empty request body",
				),
			)
		}

		txBytes, err := hex.DecodeString(rawTx.(string))
		if err != nil {
			return httpmock.NewJsonResponse(errorResponseForStatusWithExtraInfo(arcHttpStatusMalformed, err.Error()))
		}

		tx, err := sdk.NewTransactionFromBytes(txBytes)
		if err != nil {
			return httpmock.NewJsonResponse(errorResponseForStatusWithExtraInfo(arcHttpStatusMalformed, err.Error()))
		}

		if !f.verifyTxScripts(tx) {
			message := "arc error 465: inputs must have an unlocking script or an unlocker"
			return httpmock.NewJsonResponse(errorResponseForStatusWithExtraInfo(arcHttpStatusCumulativeFeeValidationFailed, message))
		}

		f.storeEFTx(tx)

		if f.broadcastWithoutResponseBody {
			return httpmock.NewJsonResponse(http.StatusOK, nil)
		} else {
			return f.getKnownTransaction(tx.TxID().String()).toResponseOrError()
		}
	})

	f.transport.RegisterResponder("GET", "=~"+f.url+"/v1/tx/.*", func(req *http.Request) (*http.Response, error) {
		txid := req.URL.String()[len(f.url+"/v1/tx/"):]
		return f.getKnownTransaction(txid).toResponse()
	})
}

func (f *arcFixture) TxInfoJSON(id string) string {
	tx := f.getKnownTransaction(id)
	require.NotNil(f, tx, "Trying to get transaction info for not existing transaction, looks like invalid test setup")

	_, content := tx.toResponseContent()
	b, err := json.Marshal(content)
	require.NoError(f, err, "failed to marshal response content")
	return string(b)
}

func (f *arcFixture) OnBroadcast() ArcBroadcastFixture {
	return f
}

func (f *arcFixture) WillReturnNoBody() {
	f.broadcastWithoutResponseBody = true
}

func (f *arcFixture) WhenQueryingTx(txID string) ARCQueryFixture {
	return &arcQueryFixture{
		TB:     f,
		parent: f,
		txID:   txID,
	}
}

func (f *arcFixture) HoldBroadcasting() ARCFixture {
	f.holdBroadcastExecution.Lock()
	return f
}

func (f *arcFixture) ReleaseBroadcasting() ARCFixture {
	f.holdBroadcastExecution.Unlock()
	return f
}

func (f *arcFixture) getKnownTransaction(txID string) *knownTransaction {
	tx, ok := f.knownTransactions.Load(txID)
	if !ok {
		return nil
	}
	return tx.(*knownTransaction)
}

func (f *arcFixture) saveKnownTransaction(tx *knownTransaction) {
	if tx == nil || tx.txid == "" {
		return
	}

	f.knownTransactions.Store(tx.txid, tx)
}

func (f *arcFixture) storeEFTx(tx *sdk.Transaction) {
	var err error
	merklePath := tx.MerklePath
	txID := tx.TxID().String()

	knownTx := &knownTransaction{
		txid:   txID,
		status: "SEEN_ON_NETWORK",
	}

	if merklePath != nil {
		knownTx.blockHash, err = merklePath.ComputeRootHex(&txID)
		require.NoError(f, err, "failed to compute root: wrong test setup")

		knownTx.status = "MINED"
		knownTx.blockHeight = merklePath.BlockHeight
		knownTx.merklePath = merklePath.Hex()
	}

	existing := f.getKnownTransaction(txID)
	if existing == nil {
		f.saveKnownTransaction(knownTx)
	}
}

func (f *arcFixture) verifyTxScripts(tx *sdk.Transaction) (isValid bool) {
	defer func() {
		if !isValid {
			f.Logf("DEBUG DATA ON SCRIPT VERIFICATION FAILURE")
			for vin, input := range tx.Inputs {
				if input.UnlockingScript == nil || len(*input.UnlockingScript) == 0 {
					f.Logf("Transaction %s has input %d without unlocking script", tx.TxID(), vin)
				} else {
					f.Logf("Transaction %s has input %d with unlocking script: %s", tx.TxID(), vin, input.UnlockingScript.String())
				}

				if input.SourceTransaction != nil {
					utxo := input.SourceTransaction.Outputs[input.SourceTxOutIndex]
					if utxo.LockingScript != nil {
						f.Logf("Transaction %s has input %d with source transaction output locking script: %s", tx.TxID(), vin, utxo.LockingScript.String())
					} else {
						f.Logf("Transaction %s has input %d with source transaction output without locking script", tx.TxID(), vin)
					}
				} else {
					f.Logf("Transaction %s has input %d with source transaction not set", tx.TxID(), vin)
				}
			}
		}
	}()

	ok, err := spv.VerifyScripts(f.Context(), tx)
	if err != nil {
		f.Logf("script verification failed: %s", err.Error())
		return false
	}

	if !ok {
		f.Logf("Transaction %s has invalid scripts", tx.TxID())
		return false
	}
	return true
}

type arcQueryFixture struct {
	testing.TB

	parent *arcFixture
	txID   string
}

func (a *arcQueryFixture) WillReturnTransactionOnHeight(height int) {
	tx := a.knownTransaction()
	tx.status = "MINED"
	tx.blockHeight = must.ConvertToUInt32(height)
}

func (a *arcQueryFixture) WillReturnTransactionWithBlockHash(hash *chainhash.Hash) {
	tx := a.knownTransaction()
	tx.status = "MINED"
	tx.blockHash = hash.String()
}

func (a *arcQueryFixture) WillReturnTransactionWithMerklePath(path sdk.MerklePath) ARCQueryFixture {
	tx := a.knownTransaction()
	tx.status = "MINED"
	tx.merklePath = path.Hex()
	tx.blockHeight = path.BlockHeight
	tx.blockHash = TestBlockHash
	return a
}

func (a *arcQueryFixture) WillReturnWithMindedTx() ARCQueryFixture {
	merklePath := testutils.MockValidMerklePath(a.TB, a.txID, 2000)
	return a.WillReturnTransactionWithMerklePath(merklePath)
}

func (a *arcQueryFixture) WillReturnDifferentTxID() {
	tx := a.knownTransaction()
	tx.txid = a.rotatedTxIdByNumberOfChars(7)
}

func (a *arcQueryFixture) WillReturnDoubleSpending(competingTxs ...string) {
	tx := a.knownTransaction()
	tx.status = "DOUBLE_SPEND_ATTEMPTED"
	tx.competingTxs = competingTxs
}

// rotatedTxIdByNumberOfChars will return rotated txid by number of chars
// for example:
// txid: 1234567890
// rotatedTxIdByNumberOfChars(3) will return 4567890123
func (a *arcQueryFixture) rotatedTxIdByNumberOfChars(number int) string {
	start := a.txID[number:]
	end := a.txID[:number]
	rotated := start + end // storing in variable for easier debugging
	return rotated
}

func (a *arcQueryFixture) WillReturnNoBody() {
	tx := a.knownTransaction()
	tx.noBody = true
}

func (a *arcQueryFixture) WillBeUnreachable() {
	tx := a.knownTransaction()
	tx.unreachable = true
}

func (a *arcQueryFixture) WillReturnHttpStatus(httpStatus int) {
	tx := a.knownTransaction()
	tx.httpStatus = httpStatus
}

func (a *arcQueryFixture) WillReturnTransactionWithoutMerklePath() {
	tx := a.knownTransaction()
	tx.status = "SEEN_ON_NETWORK"
	tx.unreachable = false
	tx.noBody = false
}

func (a *arcQueryFixture) WillReturnTransactionWithMerklePathHex(merklePath string) {
	tx := a.knownTransaction()
	tx.status = "MINED"
	tx.merklePath = merklePath
	tx.unreachable = false
	tx.noBody = false
}

func (a *arcQueryFixture) knownTransaction() *knownTransaction {
	tx := a.parent.getKnownTransaction(a.txID)

	if tx == nil {
		tx = &knownTransaction{
			txid: a.txID,
		}
		a.parent.saveKnownTransaction(tx)
	}
	return tx
}

func errorResponseForStatus(httpStatus int) (int, map[string]any) {
	return errorResponseForStatusWithExtraInfo(httpStatus, "")
}

func errorResponseForStatusWithExtraInfo(httpStatus int, extraInfo string) (int, map[string]any) {
	title := http.StatusText(httpStatus)
	var details string
	switch httpStatus {
	case http.StatusBadRequest:
		details = "The request seems to be malformed and cannot be processed"
	case http.StatusUnauthorized:
		details = "The request is not authorized"
	case http.StatusForbidden:
		details = "The request is not authorized"
	case http.StatusNotFound:
		details = "The requested resource could not be found"
	case arcHttpStatusCumulativeFeeValidationFailed:
		details = "Fee too low"
		title = "Fees are insufficient"
	case arcHttpStatusMalformed:
		details = "Transaction is malformed and cannot be processed"
		title = "Malformed transaction"
	case http.StatusInternalServerError:
		details = "The server encountered an internal error and was unable to complete your request"
	}

	return httpStatus, map[string]any{
		"error":     details,
		"extraInfo": extraInfo,
		"instance":  nil,
		"status":    httpStatus,
		"title":     title,
		"txid":      nil,
		"type":      "https://bitcoin-sv.github.io/arc/#/errors?id=_" + to.StringFromInteger(httpStatus),
	}
}

func (a *arcQueryFixture) WillReturnTransactionWithBlockHeight(height uint32) {
	mp := testutils.MockValidMerklePath(a.TB, a.txID, height)
	a.WillReturnTransactionWithMerklePath(mp)
}
