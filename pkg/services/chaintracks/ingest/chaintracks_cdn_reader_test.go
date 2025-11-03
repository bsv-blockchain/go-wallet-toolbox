package ingest_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest/testabilities"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCDNReader_FetchBulkHeaderFilesInfo(t *testing.T) {
	// given:
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.SetTransport(transport)

	reader := ingest.NewCDNReader(logging.NewTestLogger(t), ingest.BabbageCDNBaseURL, client)

	// and:
	transport.RegisterResponder("GET", mainNetCDNURL(),
		httpmock.NewJsonResponderOrPanic(200, testabilities.BabbageCDNFilesInfo(t)))

	// when:
	info, err := reader.FetchBulkHeaderFilesInfo(t.Context(), defs.NetworkMainnet)

	// then:
	require.NoError(t, err)

	assert.Equal(t, ingest.BabbageCDNBaseURL, info.RootFolder)
	assert.Equal(t, "mainNetBlockHeaders.json", info.JSONFilename)
	assert.Equal(t, 100000, info.HeadersPerFile)
	assert.Len(t, info.Files, 10)

	firstChunk := info.Files[0]
	assert.Equal(t, "mainNet_0.headers", firstChunk.FileName)
	assert.Equal(t, uint(0), firstChunk.FirstHeight)
	assert.Equal(t, 100000, firstChunk.Count)
	assert.Equal(t, "DMXYETHMphmYRh5y0+qsJhj67ML5Ui4LE1eEZDYbnZE=", *firstChunk.FileHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", firstChunk.PrevChainWork)
	assert.Equal(t, "000000000002d01c1fccc21636b607dfd930d31d01c3a62104612a1719011250", *firstChunk.LastHash)
	assert.Equal(t, defs.NetworkMainnet, *firstChunk.Chain)
	assert.Equal(t, "https://cdn.projectbabbage.com/blockheaders", *firstChunk.SourceURL)
}

func TestCDNReader_FetchBulkHeaderFilesInfo_Errors(t *testing.T) {
	tests := map[string]struct {
		setupResponder func(transport *httpmock.MockTransport)
	}{
		"404 result": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", mainNetCDNURL(), httpmock.NewStringResponder(404, "Not Found"))
			},
		},
		"invalid JSON": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", mainNetCDNURL(), httpmock.NewStringResponder(200, "this is not json"))
			},
		},
		"empty response": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", mainNetCDNURL(), httpmock.NewStringResponder(200, ""))
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			transport := httpmock.NewMockTransport()
			client := resty.New()
			client.SetTransport(transport)

			reader := ingest.NewCDNReader(logging.NewTestLogger(t), ingest.BabbageCDNBaseURL, client)

			// and:
			test.setupResponder(transport)

			// when:
			_, err := reader.FetchBulkHeaderFilesInfo(t.Context(), defs.NetworkMainnet)

			// then:
			require.Error(t, err)
		})
	}
}

func mainNetCDNURL() string {
	return fmt.Sprintf("%s/mainNetBlockHeaders.json", ingest.BabbageCDNBaseURL)
}
