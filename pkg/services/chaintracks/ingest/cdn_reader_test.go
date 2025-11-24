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

const firstFilename = "mainNet_0.headers"

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
	assert.Equal(t, firstFilename, firstChunk.FileName)
	assert.Equal(t, uint(0), firstChunk.FirstHeight)
	assert.Equal(t, 100000, firstChunk.Count)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", firstChunk.PrevChainWork)
	assert.Equal(t, "000000000002d01c1fccc21636b607dfd930d31d01c3a62104612a1719011250", *firstChunk.LastHash)
	assert.Equal(t, defs.NetworkMainnet, firstChunk.Chain)
	assert.Equal(t, "https://cdn.projectbabbage.com/blockheaders", firstChunk.SourceURL)
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

func TestCDNReader_FetchBulkHeaderFile(t *testing.T) {
	// given:
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.SetTransport(transport)

	transport.RegisterResponder("GET", mainNetCDNURL(),
		httpmock.NewJsonResponderOrPanic(200, map[string]any{
			"rootFolder":     ingest.BabbageCDNBaseURL,
			"jsonFilename":   "mainNetBlockHeaders.json",
			"headersPerFile": 100000,
			"files": []map[string]any{
				{
					"chain":         "main",
					"count":         10,
					"fileHash":      "bE1oOTaH49qwzuG+s0T/4EAQP5+rtthBtJW9akTwRCM=",
					"fileName":      firstFilename,
					"firstHeight":   0,
					"lastChainWork": "0000000000000000000000000000000000000000000000000000000a000a000a",
					"lastHash":      "000000008d9dc510f23c2657fc4f67bea30078cc05a90eb89e84cc475c080805",
					"prevChainWork": "0000000000000000000000000000000000000000000000000000000000000000",
					"prevHash":      "0000000000000000000000000000000000000000000000000000000000000000",
					"sourceUrl":     "https://cdn.projectbabbage.com/blockheaders",
				},
			},
		}))

	transport.RegisterResponder("GET", fmt.Sprintf("%s/%s", ingest.BabbageCDNBaseURL, firstFilename),
		httpmock.NewBytesResponder(200, testabilities.First10HeadersData))

	// and:
	reader := ingest.NewCDNReader(logging.NewTestLogger(t), ingest.BabbageCDNBaseURL, client)

	// when:
	info, err := reader.FetchBulkHeaderFilesInfo(t.Context(), defs.NetworkMainnet)
	require.NoError(t, err)

	data, err := reader.DownloadBulkHeaderFile(t.Context(), info.Files[0].FileName)

	// then:
	require.NoError(t, err)
	bulkFileData := ingest.BulkFileData{Data: data, Info: info.Files[0]}
	require.NoError(t, bulkFileData.Validate())
	require.Equal(t, 10, bulkFileData.Len())

	// when:
	header, err := ingest.GetHeaderAtIndex(bulkFileData.Data, 9, bulkFileData.Info.FirstHeight)
	require.NoError(t, err)

	// then:
	assert.Equal(t, "000000008d9dc510f23c2657fc4f67bea30078cc05a90eb89e84cc475c080805", header.Hash)
	assert.Equal(t, uint(9), header.Height)
	assert.Equal(t, "00000000408c48f847aa786c2268fc3e6ec2af68e8468a34a28c61b7f1de0dc6", header.PreviousHash)
}

func TestCDNReader_FetchBulkHeaderFile_Errors(t *testing.T) {
	tests := map[string]struct {
		setupResponder func(transport *httpmock.MockTransport)
	}{
		"404 result": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", fmt.Sprintf("%s/%s", ingest.BabbageCDNBaseURL, firstFilename),
					httpmock.NewStringResponder(404, "Not Found"))
			},
		},
		"invalid data": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", fmt.Sprintf("%s/%s", ingest.BabbageCDNBaseURL, firstFilename),
					httpmock.NewStringResponder(200, "this is not valid header data"))
			},
		},
		"empty response": {
			setupResponder: func(transport *httpmock.MockTransport) {
				transport.RegisterResponder("GET", fmt.Sprintf("%s/%s", ingest.BabbageCDNBaseURL, firstFilename),
					httpmock.NewStringResponder(200, ""))
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			transport := httpmock.NewMockTransport()
			client := resty.New()
			client.SetTransport(transport)

			// and:
			test.setupResponder(transport)

			// and:
			reader := ingest.NewCDNReader(logging.NewTestLogger(t), ingest.BabbageCDNBaseURL, client)

			// when:
			data, err := reader.DownloadBulkHeaderFile(t.Context(), firstFilename)

			// then:
			if err == nil {
				bulkFileData := ingest.BulkFileData{Data: data, Info: ingest.BulkHeaderFileInfo{}}
				err = bulkFileData.Validate()
			}
			require.Error(t, err)
		})
	}
}

func mainNetCDNURL() string {
	return fmt.Sprintf("%s/mainNetBlockHeaders.json", ingest.BabbageCDNBaseURL)
}
