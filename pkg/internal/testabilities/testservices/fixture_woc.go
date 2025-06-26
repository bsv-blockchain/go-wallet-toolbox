package testservices

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type WhatsOnChainFixture interface {
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
	OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption)
	OnTipBlockHeaderWillRespondWithEmptyList()
	WillBeUnreachable() error
	WillRespondWithInternalFailure()
	WillRespondWithMerklePath(status int, txID string, responseBody string)
	WillRespondWithBlockHeader(status int, blockHash string, responseBody string)
	WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture
	WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture
	WillRespondWithBroadcast(status int, responseBody string, err error)
	WillAlwaysReturnPostBEEFSuccess(txids ...string)
	Transport() *httpmock.MockTransport
	HttpClient() *resty.Client
}

type wocFixture struct {
	testing.TB
	transport *httpmock.MockTransport
	network   defs.BSVNetwork
}

func NewWoCFixture(t testing.TB, opts ...Option) WhatsOnChainFixture {
	options := to.OptionsWithDefault(FixtureOptions{
		network:   defs.NetworkMainnet,
		transport: httpmock.NewMockTransport(),
	}, opts...)

	return &wocFixture{
		TB:        t,
		transport: options.transport,
		network:   options.network,
	}
}

func (f *wocFixture) WillRespondWithInternalFailure() {
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithEmptyList() {
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{}),
	)
}

type TipBlockHeaderOptions struct {
	Height uint
}

type TipBlockHeaderOption = func(*TipBlockHeaderOptions)

func WithTipBlockHeaderHeight(height uint) TipBlockHeaderOption {
	return func(opts *TipBlockHeaderOptions) {
		opts.Height = height
	}
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption) {
	f.TB.Helper()

	options := to.OptionsWithDefault(TipBlockHeaderOptions{
		Height: TestBlockHeight,
	}, opts...)

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{
			{
				Hash:              TestBlockHash,
				Confirmations:     TestBlockConfirmations,
				Size:              TestBlockSize,
				Height:            options.Height,
				Version:           TestBlockVersion,
				VersionHex:        TestBlockVersionHex,
				MerkleRoot:        TestBlockMerkleRoot,
				Time:              TestBlockTime,
				MedianTime:        TestBlockMedianTime,
				Nonce:             TestBlockNonce,
				Bits:              TestBlockBits,
				Difficulty:        TestBlockDifficulty,
				ChainWork:         TestBlockChainWork,
				PreviousBlockHash: TestBlockPreviousBlockHash,
				NextBlockHash:     nil,
				NTx:               TestBlockNTx,
				NumTx:             TestBlockNumTx,
			},
		}),
	)
}

func (f *wocFixture) WillBeUnreachable() error {
	err := net.UnknownNetworkError("tests defined this endpoint as unreachable")
	f.TB.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`^https://api\.whatsonchain\.com/.*`),
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (f *wocFixture) WillRespondWithRates(status int, content string, err error) {
	f.TB.Helper()
	responder := func(status int, content string) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			res := httpmock.NewStringResponse(status, content)
			res.Header.Set("Content-Type", "application/json")
			return res, nil
		}
	}

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/exchangerate", f.network),
		responder(status, content),
	)
}

func (f *wocFixture) WillRespondWithRawTx(status int, txID, rawTx string, err error) {
	f.TB.Helper()
	responder := func(status int, content string) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			res := httpmock.NewStringResponse(status, content)
			res.Header.Set("Content-Type", "text/plain")
			return res, nil
		}
	}

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", f.network, txID),
		responder(status, rawTx),
	)
}

const (
	TestBlockConfirmations = 1
	TestBlockSize          = 2184411
	TestBlockHeight        = 901475
	TestBlockNTx           = 0
	TestBlockNumTx         = 3196
)

const (
	TestBlockVersion    uint32 = 805306368
	TestBlockNonce      uint32 = 602597547
	TestBlockTime       uint64 = 1750064695
	TestBlockMedianTime uint64 = 1750060569
)

const (
	TestBlockDifficulty = 64454475829.11144
)

const (
	TestBlockVersionHex        = "30000000"
	TestBlockMerkleRoot        = "c7a78f2edd611b0fe7aad6829a243e4a9e351e5ab203b7beb875ba1e6a80249e"
	TestBlockBits              = "18110ef8"
	TestBlockChainWork         = "000000000000000000000000000000000000000001669c7b159861f30c53271e"
	TestBlockPreviousBlockHash = "000000000000000001885e0c6c302cbbacf927e1b5cf7884588973e72f8b704e"
)

type wocBlockResponseItem struct {
	Hash              string  `json:"hash"`
	Confirmations     int     `json:"confirmations"`
	Size              int     `json:"size"`
	Height            uint    `json:"height"`
	Version           uint32  `json:"version"`
	VersionHex        string  `json:"versionHex"`
	MerkleRoot        string  `json:"merkleroot"`
	Time              uint64  `json:"time"`
	MedianTime        uint64  `json:"mediantime"`
	Nonce             uint32  `json:"nonce"`
	Bits              string  `json:"bits"`
	Difficulty        float64 `json:"difficulty"`
	ChainWork         string  `json:"chainwork"`
	PreviousBlockHash string  `json:"previousblockhash"`
	NextBlockHash     *string `json:"nextblockhash,omitempty"`
	NTx               int     `json:"nTx"`
	NumTx             int     `json:"num_tx"`
}

func (f *wocFixture) WillRespondWithMerklePath(status int, txID, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/proof/tsc", f.network, txID)
	f.transport.RegisterResponder("GET", url, responder)
}

func (f *wocFixture) WillRespondWithBlockHeader(status int, blockHash, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/%s/header", f.network, blockHash)
	f.transport.RegisterResponder("GET", url, responder)
}

func (f *wocFixture) WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture {
	return &wocMerklePathQueryFixture{fixture: f, txID: txID}
}

func (f *wocFixture) WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture {
	return &wocBlockHeaderQueryFixture{fixture: f, blockHash: blockHash}
}

func (f *wocFixture) Transport() *httpmock.MockTransport {
	return f.transport
}

func (f *wocFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.GetClient().Transport = f.transport
	return client
}

func (f *wocFixture) WillRespondWithBroadcast(status int, responseBody string, err error) {
	responder := func(req *http.Request) (*http.Response, error) {
		if err != nil {
			return nil, err
		}
		res := httpmock.NewStringResponse(status, responseBody)
		res.Header.Set("Content-Type", "application/json")
		return res, nil
	}

	url := mockBroadcastURL(f.network)
	f.transport.RegisterResponder("POST", url, responder)
}

func (f *wocFixture) WillAlwaysReturnPostBEEFSuccess(txids ...string) {
	f.Transport().RegisterResponder("POST", mockBroadcastURL(f.network), func(req *http.Request) (*http.Response, error) {
		var body struct {
			TxHex string `json:"txhex"`
		}
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusBadRequest, "bad request"), nil
		}

		rawTx, err := hex.DecodeString(body.TxHex)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusBadRequest, "invalid hex"), nil
		}

		computedTxid := computeTxID(rawTx)

		for _, txid := range txids {
			if txid == computedTxid {
				respBody := fmt.Sprintf(`{"txid":"%s"}`, txid)
				resp := httpmock.NewStringResponse(http.StatusOK, respBody)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
		}

		return httpmock.NewStringResponse(http.StatusBadRequest, "txid not found"), nil
	})
}

func mockBroadcastURL(network defs.BSVNetwork) string {
	return fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/raw", network)
}

// computeTxID takes raw transaction bytes and returns the transaction ID (txid) as string.
func computeTxID(rawTx []byte) string {
	tx, err := transaction.NewTransactionFromBytes(rawTx)
	if err != nil {
		return ""
	}
	return tx.TxID().String()
}
