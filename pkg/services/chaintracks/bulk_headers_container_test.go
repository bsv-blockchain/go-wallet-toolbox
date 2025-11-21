package chaintracks

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkHeadersContainer_Add(t *testing.T) {
	const chunkSize = 3
	const filePrefix = "mainNet"
	chain := defs.NetworkMainnet
	headersData := testabilities.First10HeadersData
	testData := splitTestDataIntoChunks(headersData, chunkSize)

	container := newBulkHeadersContainer(logging.NewTestLogger(t), chunkSize)

	for _, chunk := range testData {
		err := container.Add(t.Context(), chunk.data, models.NewHeightRange(chunk.firstHeight, chunk.firstHeight+uint(chunk.count)-1))

		require.NoError(t, err)

		currentRange := container.Range()
		assert.Equal(t, uint(0), currentRange.MinHeight)
		assert.Equal(t, chunk.firstHeight+uint(chunk.count)-1, currentRange.MaxHeight)

		err = container.Update(chain, filePrefix)
		require.NoError(t, err)
	}

	for i := range 10 {
		header, err := container.FindHeaderForHeight(uint(i))

		require.NoError(t, err)

		originalData := headersData[i*80 : (i+1)*80]
		retrievedData, err := header.Bytes()

		require.NoError(t, err)
		assert.Equal(t, originalData, retrievedData, "header data should match for height %d", i)
	}

	require.Equal(t, 4, len(container.GeneratedFileData))
	fileData := container.GeneratedFileData[0]
	assert.NoError(t, fileData.Validate())
	assert.Equal(t, uint(0), fileData.Info.FirstHeight)
	assert.Equal(t, chunkSize, fileData.Info.Count)
	assert.Equal(t, chain, fileData.Info.Chain)
	assert.Equal(t, filePrefix+"_0.headers", fileData.Info.FileName)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", fileData.Info.PrevChainWork)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", fileData.Info.PrevHash)
	assert.Equal(t, "000000006a625f06636b8bb6ac7b960a8d03705d1ace08b1a19da3fdcc99ddbd", *fileData.Info.LastHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000300030003", fileData.Info.LastChainWork)
	fileData = container.GeneratedFileData[1]
	assert.NoError(t, fileData.Validate())
	assert.Equal(t, uint(3), fileData.Info.FirstHeight)
	assert.Equal(t, chunkSize, fileData.Info.Count)
	assert.Equal(t, chain, fileData.Info.Chain)
	assert.Equal(t, filePrefix+"_1.headers", fileData.Info.FileName)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000300030003", fileData.Info.PrevChainWork)
	assert.Equal(t, "000000006a625f06636b8bb6ac7b960a8d03705d1ace08b1a19da3fdcc99ddbd", fileData.Info.PrevHash)
	assert.Equal(t, "000000009b7262315dbf071787ad3656097b892abffd1f95a1a022f896f533fc", *fileData.Info.LastHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000600060006", fileData.Info.LastChainWork)
	fileData = container.GeneratedFileData[2]
	assert.NoError(t, fileData.Validate())
	assert.Equal(t, uint(6), fileData.Info.FirstHeight)
	assert.Equal(t, chunkSize, fileData.Info.Count)
	assert.Equal(t, chain, fileData.Info.Chain)
	assert.Equal(t, filePrefix+"_2.headers", fileData.Info.FileName)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000600060006", fileData.Info.PrevChainWork)
	assert.Equal(t, "000000009b7262315dbf071787ad3656097b892abffd1f95a1a022f896f533fc", fileData.Info.PrevHash)
	assert.Equal(t, "00000000408c48f847aa786c2268fc3e6ec2af68e8468a34a28c61b7f1de0dc6", *fileData.Info.LastHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000900090009", fileData.Info.LastChainWork)
	fileData = container.GeneratedFileData[3]
	assert.NoError(t, fileData.Validate())
	assert.Equal(t, uint(9), fileData.Info.FirstHeight)
	assert.Equal(t, 1, fileData.Info.Count)
	assert.Equal(t, chain, fileData.Info.Chain)
	assert.Equal(t, filePrefix+"_3.headers", fileData.Info.FileName)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000900090009", fileData.Info.PrevChainWork)
	assert.Equal(t, "00000000408c48f847aa786c2268fc3e6ec2af68e8468a34a28c61b7f1de0dc6", fileData.Info.PrevHash)
	assert.Equal(t, "000000008d9dc510f23c2657fc4f67bea30078cc05a90eb89e84cc475c080805", *fileData.Info.LastHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000a000a000a", fileData.Info.LastChainWork)
}

func TestBulkHeadersContainer_Add_BigChunkSize(t *testing.T) {
	const chunkSize = 20
	const filePrefix = "mainNet"
	chain := defs.NetworkMainnet
	headersData := testabilities.First10HeadersData
	testData := splitTestDataIntoChunks(headersData, chunkSize)

	container := newBulkHeadersContainer(logging.NewTestLogger(t), chunkSize)

	for _, chunk := range testData {
		err := container.Add(t.Context(), chunk.data, models.NewHeightRange(chunk.firstHeight, chunk.firstHeight+uint(chunk.count)-1))

		require.NoError(t, err)

		currentRange := container.Range()
		assert.Equal(t, uint(0), currentRange.MinHeight)
		assert.Equal(t, chunk.firstHeight+uint(chunk.count)-1, currentRange.MaxHeight)

		err = container.Update(chain, filePrefix)
		require.NoError(t, err)
		assert.Equal(t, 1, len(container.GeneratedFileData))
	}

	require.Equal(t, 1, len(container.chunks), "there should be only one chunk when chunk size exceeds total headers")

	fileData := container.GeneratedFileData[0]
	assert.NoError(t, fileData.Validate())
	assert.Equal(t, uint(0), fileData.Info.FirstHeight)
	assert.Equal(t, 10, fileData.Info.Count)
	assert.Equal(t, chain, fileData.Info.Chain)
	assert.Equal(t, "mainNet_0.headers", fileData.Info.FileName)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", fileData.Info.PrevChainWork)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", fileData.Info.PrevHash)
	assert.Equal(t, "000000008d9dc510f23c2657fc4f67bea30078cc05a90eb89e84cc475c080805", *fileData.Info.LastHash)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000a000a000a", fileData.Info.LastChainWork)
}

type testChunkData struct {
	firstHeight uint
	count       int
	data        []byte
}

func splitTestDataIntoChunks(headersData []byte, chunkSize int) []*testChunkData {
	var chunks []*testChunkData
	totalHeaders := len(headersData) / 80
	for i := range totalHeaders {
		if i%chunkSize == 0 {
			chunks = append(chunks, &testChunkData{
				firstHeight: uint(i),
				count:       0,
				data:        make([]byte, 0),
			})
		}
		currentChunk := chunks[len(chunks)-1]
		currentChunk.data = append(currentChunk.data, headersData[i*80:(i+1)*80]...)
		currentChunk.count++
	}
	return chunks
}
