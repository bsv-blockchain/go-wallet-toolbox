package testabilities

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ArcURL = "https://api.taal.com/arc"
const ArcToken = "mainnet_9596de07e92300c6287e4393594ae39c"
const DeploymentID = "go-wallet-toolbox-test"
const arcHttpStatusMalformed = 463

var timestamp = time.Date(2018, time.November, 10, 23, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.999999999Z")

type ARCFixture interface {
	IsUpAndRunning()
	HttpClient() *resty.Client
	TxInfoJSON(id string) string
	WillAlwaysReturnStatus(httpStatus int)
	WhenQueryingTx(txID string) ARCQueryFixture
	OnBroadcast() ArcBroadcastFixture
}

type ARCQueryFixture interface {
	WillReturnHttpStatus(httpStatus int)
	WillBeUnreachable()
	WillReturnNoBody()
	WillReturnDifferentTxID()
}

type ArcBroadcastFixture interface {
	WillReturnNoBody()
}

type arcFixture struct {
	testing.TB
	transport                    *httpmock.MockTransport
	knownTransactions            map[string]*knownTransaction
	broadcastWithoutResponseBody bool
}

func NewARCFixture(t testing.TB) ARCFixture {
	transport := httpmock.NewMockTransport()
	return NewArcFixtureWithTransport(t, transport)
}

func NewArcFixtureWithTransport(t testing.TB, transport *httpmock.MockTransport) ARCFixture {
	require.NotNil(t, transport, "http.RoundTripper must be provided")

	return &arcFixture{
		TB:                t,
		transport:         transport,
		knownTransactions: make(map[string]*knownTransaction),
	}
}

func (f *arcFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.SetTransport(f.transport)
	return client
}

func (f *arcFixture) WillAlwaysReturnStatus(httpStatus int) {
	f.transport.RegisterResponder("POST", "=~"+ArcURL+"/v1/tx.*", func(req *http.Request) (*http.Response, error) {
		return httpmock.NewJsonResponse(errorResponseForStatus(httpStatus))
	})
}

func (f *arcFixture) IsUpAndRunning() {
	f.transport.RegisterResponder(http.MethodPost, ArcURL+"/v1/tx", func(req *http.Request) (*http.Response, error) {
		b, err := io.ReadAll(req.Body)
		if !assert.NoError(f, err) {
			return nil, err
		}

		var body map[string]any
		err = json.Unmarshal(b, &body)
		if !assert.NoError(f, err) {
			return nil, err
		}

		rawTx := body["rawTx"]
		if !assert.NotNil(f, rawTx) {
			return httpmock.NewJsonResponse(
				errorResponseForStatusWithExtraInfo(
					http.StatusBadRequest,
					"error parsing transactions from request: no transaction found - empty request body",
				),
			)
		}
		txHex := rawTx.(string)

		tx, err := sdk.NewTransactionFromBEEFHex(txHex)
		if !assert.NoError(f, err) {
			return httpmock.NewJsonResponse(errorResponseForStatusWithExtraInfo(arcHttpStatusMalformed, err.Error()))
		}

		f.store(txHex)

		if f.broadcastWithoutResponseBody {
			return httpmock.NewJsonResponse(http.StatusOK, nil)
		} else {
			return f.knownTransactions[tx.TxID().String()].toResponseOrError()
		}

	})

	f.transport.RegisterResponder("GET", "=~"+ArcURL+"/v1/tx/.*", func(req *http.Request) (*http.Response, error) {
		txid := req.URL.String()[len(ArcURL+"/v1/tx/"):]
		return f.knownTransactions[txid].toResponse()
	})
}

func (f *arcFixture) TxInfoJSON(id string) string {
	tx, ok := f.knownTransactions[id]
	require.True(f, ok, "Trying to get transaction info for not existing transaction, looks like invalid test setup")

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

func (f *arcFixture) store(txHex string) {
	beefBytes, err := hex.DecodeString(txHex)
	require.NoError(f, err, "failed to decode BEEF hex")

	beef, err := sdk.NewBeefFromBytes(beefBytes)
	require.NoError(f, err, "failed to create BEEF from bytes")

	transactions := seq2.FromMap(beef.Transactions)
	includedTransactions := seq2.MapTo(transactions, func(txID string, tx *sdk.BeefTx) *knownTransaction {
		merklePath := optional.OfPtr(tx.Transaction.MerklePath)

		return &knownTransaction{
			txid:        txID,
			status:      to.IfThen(merklePath.IsEmpty(), "SEEN_ON_NETWORK").ElseThen("MINED"),
			blockHeight: optional.Map(merklePath, func(it sdk.MerklePath) uint32 { return it.BlockHeight }).OrZeroValue(),
			blockHash: optional.Map(merklePath, func(it sdk.MerklePath) string {
				root, err := it.ComputeRootHex(&txID)
				require.NoError(f, err, "failed to compute root: wrong test setup")
				return root
			}).OrZeroValue(),
			merklePath: optional.Map(merklePath, func(it sdk.MerklePath) string { return it.Hex() }).OrZeroValue(),
		}
	})

	seq.ForEach(includedTransactions, func(it *knownTransaction) {
		if _, ok := f.knownTransactions[it.txid]; !ok {
			f.knownTransactions[it.txid] = it
		}
	})
}

type arcQueryFixture struct {
	testing.TB
	parent *arcFixture
	txID   string
}

func (a *arcQueryFixture) WillReturnDifferentTxID() {
	tx := a.knownTransaction()
	tx.txid = a.rotatedTxIdByNumberOfChars(7)
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

func (a *arcQueryFixture) knownTransaction() *knownTransaction {
	tx, ok := a.parent.knownTransactions[a.txID]
	if !ok {
		tx = &knownTransaction{
			txid: a.txID,
		}
		a.parent.knownTransactions[a.txID] = tx
	}
	return tx
}

func errorResponseForStatus(httpStatus int) (int, map[string]any) {
	return errorResponseForStatusWithExtraInfo(httpStatus, "")
}

func errorResponseForStatusWithExtraInfo(httpStatus int, extraInfo string) (int, map[string]any) {
	var title = http.StatusText(httpStatus)
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
