package testservices

import (
	"fmt"
	"net/http"

	"github.com/jarcoal/httpmock"
)

type WhatsOnChainMerklePathQueryFixture interface {
	WillReturnMerklePathResponse(status int, responseBody string)
	WillReturnTSCProof(status int, tscProofJSON string)
}

type wocMerklePathQueryFixture struct {
	fixture *wocFixture
	txID    string
}

func (q *wocMerklePathQueryFixture) WillReturnMerklePathResponse(status int, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/proof/tsc", q.fixture.network, q.txID)
	q.fixture.transport.RegisterResponder("GET", url, responder)
}

func (q *wocMerklePathQueryFixture) WillReturnTSCProof(status int, tscProofJSON string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, tscProofJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/proof/tsc", q.fixture.network, q.txID)
	q.fixture.transport.RegisterResponder("GET", url, responder)
}
