package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

const (
	genesisAsPrevBlockHash  = "0000000000000000000000000000000000000000000000000000000000000000"
	prevChainWorkForGenesis = "0000000000000000000000000000000000000000000000000000000000000000"
)

type chunkData struct {
	PrevChainWork internal.ChainWork
	LastChainWork internal.ChainWork
	LastHash      string
	data          []byte
}

type bulkHeadersContainer struct {
	logger    *slog.Logger
	chunkSize int
	chunks    []chunkData

	GeneratedFileData []ingest.BulkFileData
	chain             defs.BSVNetwork
}

func newBulkHeadersContainer(logger *slog.Logger, chunkSize int, chain defs.BSVNetwork) *bulkHeadersContainer {
	logger = logging.Child(logger, "bulk_headers_container")
	return &bulkHeadersContainer{
		logger:    logger,
		chunkSize: chunkSize,
		chunks:    make([]chunkData, 0),
		chain:     chain,
	}
}

func (b *bulkHeadersContainer) Add(ctx context.Context, data []byte, dataRange models.HeightRange) error {
	currentRange := b.Range()
	rangeToAdd := dataRange.Above(currentRange)
	if rangeToAdd.IsEmpty() {
		b.logger.Debug("Skipping addition of bulk headers - no new headers to add", slog.Any("data_range", dataRange), slog.Any("current_range", currentRange))
		return nil
	}

	if currentRange.NotEmpty() && currentRange.MaxHeight+1 != rangeToAdd.MinHeight {
		return fmt.Errorf("cannot add bulk headers - non-contiguous range detected, current range: %v, range to add: %v", currentRange, rangeToAdd)
	}

	dataToAdd, err := b.trimToRange(data, dataRange, rangeToAdd)
	if err != nil {
		return fmt.Errorf("failed to trim bulk file chunks to range %v: %w", rangeToAdd, err)
	}

	count := len(dataToAdd) / 80

	for i := range count {
		unsignedIndex := must.ConvertToUInt(i)
		if ctx.Err() != nil {
			return fmt.Errorf("operation canceled while adding bulk headers: %w", ctx.Err())
		}

		height := must.ConvertToIntFromUnsigned(dataRange.MinHeight) + i
		chunkIndex := b.getIndexForHeight(height)
		header, err := ingest.GetHeaderAtIndex(dataToAdd, unsignedIndex, dataRange.MinHeight)
		if err != nil {
			return fmt.Errorf("failed to get header chunks at index %d: %w", i, err)
		}

		headerData, _ := ingest.GetHeaderDataAtIndex(dataToAdd, unsignedIndex)

		if chunkIndex >= len(b.chunks) {
			// Create new chunk
			prevChainWork := internal.ChainWork{}
			if len(b.chunks) > 0 {
				prevChainWork = b.chunks[len(b.chunks)-1].LastChainWork
			}

			headerChainWork := internal.ChainWorkFromBits(header.Bits)
			chainWork := headerChainWork.AddChainWork(prevChainWork)

			chunk := chunkData{
				PrevChainWork: prevChainWork,
				LastChainWork: chainWork,
				LastHash:      header.Hash,
				data:          make([]byte, 0, b.chunkSize*80),
			}
			chunk.data = append(chunk.data, headerData...)

			b.chunks = append(b.chunks, chunk)
		} else {
			// Append to existing chunk
			chunk := &b.chunks[chunkIndex]

			headerChainWork := internal.ChainWorkFromBits(header.Bits)

			chainWork := headerChainWork.AddChainWork(chunk.LastChainWork)

			chunk.LastChainWork = chainWork
			chunk.LastHash = header.Hash
			chunk.data = append(chunk.data, headerData...)
		}
	}

	return nil
}

func (b *bulkHeadersContainer) Range() models.HeightRange {
	length := len(b.chunks)
	if length == 0 {
		return models.NewEmptyHeightRange()
	}

	return models.NewHeightRange(0, must.ConvertToUInt(b.MaxHeightAtChunk(length-1)))
}

func (b *bulkHeadersContainer) MaxHeightAtChunk(index int) int {
	if index >= len(b.chunks) {
		return 0
	}

	return index*b.chunkSize + len(b.chunks[index].data)/80 - 1
}

func (b *bulkHeadersContainer) FindHeaderForHeight(height uint) (*wdk.ChainBlockHeader, error) {
	chunkIndex := b.getIndexForHeight(must.ConvertToIntFromUnsigned(height))
	if chunkIndex >= len(b.chunks) {
		return nil, nil
	}

	chunk := b.chunks[chunkIndex]
	headerIndexInChunk := must.ConvertToIntFromUnsigned(height) - chunkIndex*b.chunkSize

	headerDataStart := headerIndexInChunk * 80
	headerDataEnd := headerDataStart + 80

	if headerDataEnd > len(chunk.data) {
		return nil, fmt.Errorf("header data end index %d exceeds chunk data length %d", headerDataEnd, len(chunk.data))
	}

	headerData := chunk.data[headerDataStart:headerDataEnd]

	baseBlockHeader, err := wdk.ChainBaseBlockHeaderFromBytes(headerData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block header at height %d: %w", height, err)
	}

	blockHash, err := baseBlockHeader.CalculateHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for block header at height %d: %w", height, err)
	}

	header := &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: *baseBlockHeader,
		Height:               height,
		Hash:                 blockHash.String(),
	}

	return header, nil
}

func (b *bulkHeadersContainer) Update() error {
	b.GeneratedFileData = make([]ingest.BulkFileData, 0, len(b.chunks))

	for i, chunk := range b.chunks {
		prevChainWorkHex := prevChainWorkForGenesis
		prevHash := genesisAsPrevBlockHash
		if i > 0 {
			prevChainWorkHex = chunk.PrevChainWork.To64PadHex()
			prevHash = b.chunks[i-1].LastHash
		}

		fileInfo := ingest.BulkHeaderFileInfo{
			BulkHeaderMinimumInfo: ingest.BulkHeaderMinimumInfo{
				FileName:    fmt.Sprintf("%sNet_%d.headers", b.chain, i),
				FirstHeight: must.ConvertToUInt(i * b.chunkSize),
				Count:       len(chunk.data) / 80,
			},
			PrevChainWork: prevChainWorkHex,
			LastChainWork: chunk.LastChainWork.To64PadHex(),
			PrevHash:      prevHash,
			LastHash:      &chunk.LastHash,
			Chain:         b.chain,
		}

		now := time.Now()
		fileData := ingest.BulkFileData{
			Info:      fileInfo,
			Data:      make([]byte, len(chunk.data)),
			UpdatedAt: now,
		}
		copy(fileData.Data, chunk.data)

		fileData.Info.FileHash = fileData.Hash()

		b.GeneratedFileData = append(b.GeneratedFileData, fileData)
	}

	return nil
}

func (b *bulkHeadersContainer) getIndexForHeight(height int) int {
	return height / b.chunkSize
}

func (b *bulkHeadersContainer) trimToRange(data []byte, initialHeightRange, desiredHeightRange models.HeightRange) ([]byte, error) {
	if desiredHeightRange.IsEmpty() {
		return nil, fmt.Errorf("cannot trim bulk file data to an empty height range")
	}

	intersection := initialHeightRange.Intersect(desiredHeightRange)
	if intersection.IsEmpty() {
		return nil, fmt.Errorf("no overlap between bulk file range %v and target range %v", initialHeightRange, desiredHeightRange)
	}

	startIndex := intersection.MinHeight - initialHeightRange.MinHeight
	endIndex := intersection.MaxHeight - initialHeightRange.MinHeight

	startByte := startIndex * 80
	endByte := (endIndex + 1) * 80

	return data[startByte:endByte], nil
}

func (b *bulkHeadersContainer) FilesInfo() []ingest.BulkHeaderFileInfo {
	filesInfo := make([]ingest.BulkHeaderFileInfo, 0, len(b.GeneratedFileData))
	for _, fileData := range b.GeneratedFileData {
		filesInfo = append(filesInfo, fileData.Info)
	}

	return filesInfo
}

func (b *bulkHeadersContainer) GetFileDataByIndex(fileID int) (*ingest.BulkFileData, error) {
	if fileID < 0 || fileID >= len(b.GeneratedFileData) {
		return nil, fmt.Errorf("file ID %d is out of range", fileID)
	}

	return &b.GeneratedFileData[fileID], nil
}
