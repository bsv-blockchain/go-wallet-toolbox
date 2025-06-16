package testservices

import (
	"fmt"
	"net/http"

	"github.com/jarcoal/httpmock"
)

type WhatsOnChainBlockHeaderQueryFixture interface {
	WillReturnBlockHeaderResponse(status int, responseBody string)
	WillReturnBlockHeaderJSON(status int, blockHeaderJSON string)
}

type wocBlockHeaderQueryFixture struct {
	fixture   *wocFixture
	blockHash string
}

func (q *wocBlockHeaderQueryFixture) WillReturnBlockHeaderResponse(status int, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/%s/header", q.fixture.network, q.blockHash)
	q.fixture.transport.RegisterResponder("GET", url, responder)
}

func (q *wocBlockHeaderQueryFixture) WillReturnBlockHeaderJSON(status int, blockHeaderJSON string) {
	q.WillReturnBlockHeaderResponse(status, blockHeaderJSON)
}
