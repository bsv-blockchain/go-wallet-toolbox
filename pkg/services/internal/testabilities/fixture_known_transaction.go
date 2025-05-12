package testabilities

import (
	"fmt"
	"net"
	"net/http"

	"github.com/jarcoal/httpmock"
)

type knownTransaction struct {
	txid        string
	status      string
	blockHeight uint32
	blockHash   string
	merklePath  string
	httpStatus  int
	unreachable bool
	noBody      bool
}

func (t *knownTransaction) toResponse() (*http.Response, error) {
	if t != nil && t.unreachable {
		return nil, net.UnknownNetworkError("tests defined this endpoint for tx as unreachable")
	}
	return httpmock.NewJsonResponse(t.toResponseContent())
}

func (t *knownTransaction) toResponseOrError() (*http.Response, error) {
	if t == nil {
		return nil, fmt.Errorf("unexpectedly cannot find transaction in known transactions")
	}
	return t.toResponse()
}

func (t *knownTransaction) toResponseContent() (int, map[string]any) {
	if t == nil {
		return errorResponseForStatusWithExtraInfo(404, "transaction not found")
	}

	if t.httpStatus > 0 && t.httpStatus != 200 {
		return errorResponseForStatus(t.httpStatus)
	}

	if t.noBody {
		return http.StatusOK, nil
	}

	return http.StatusOK, map[string]any{
		"blockHash":    t.blockHash,
		"blockHeight":  t.blockHeight,
		"competingTxs": nil,
		"extraInfo":    "",
		"merklePath":   t.merklePath,
		"timestamp":    timestamp,
		"txStatus":     t.status,
		"txid":         t.txid,
	}
}
