package whatsonchain

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/go-softwarelab/common/pkg/to"
	wocsdk "github.com/mrz1836/go-whatsonchain"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// maxTxsPerStatusRequest is the maximum number of txIDs WhatsOnChain accepts in a
// single /txs/status request.
const maxTxsPerStatusRequest = 20

// blockInfoToChainBaseBlockHeader converts an SDK BlockInfo into a wdk.ChainBaseBlockHeader.
func blockInfoToChainBaseBlockHeader(b *wocsdk.BlockInfo) (*wdk.ChainBaseBlockHeader, error) {
	bits, err := strconv.ParseUint(b.Bits, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %q: expected hex string convertible to uint32: %w", b.Bits, err)
	}

	version, err := to.UInt32(b.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version %d: %w", b.Version, err)
	}

	blockTime, err := to.UInt32(b.Time)
	if err != nil {
		return nil, fmt.Errorf("invalid time %d: %w", b.Time, err)
	}

	nonce, err := to.UInt32(b.Nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce %d: %w", b.Nonce, err)
	}

	return &wdk.ChainBaseBlockHeader{
		Version:      version,
		PreviousHash: b.PreviousBlockHash,
		MerkleRoot:   b.MerkleRoot,
		Time:         blockTime,
		Bits:         uint32(bits),
		Nonce:        nonce,
	}, nil
}

// blockInfoToChainBlockHeader converts an SDK BlockInfo into a wdk.ChainBlockHeader.
func blockInfoToChainBlockHeader(b *wocsdk.BlockInfo) (*wdk.ChainBlockHeader, error) {
	base, err := blockInfoToChainBaseBlockHeader(b)
	if err != nil {
		return nil, fmt.Errorf("failed to convert BlockInfo to ChainBaseBlockHeader: %w", err)
	}

	height, err := to.UInt(b.Height)
	if err != nil {
		return nil, fmt.Errorf("invalid block height %d: %w", b.Height, err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: *base,
		Height:               height,
		Hash:                 b.Hash,
	}, nil
}

// scriptRecordsToUtxoDetails maps SDK script records to wdk UTXO details.
func scriptRecordsToUtxoDetails(records wocsdk.ScriptList) []wdk.UtxoDetail {
	details := make([]wdk.UtxoDetail, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		index, err := to.UInt32(record.TxPos)
		if err != nil {
			index = 0
		}
		satoshis, err := to.UInt64(record.Value)
		if err != nil {
			satoshis = 0
		}
		details = append(details, wdk.UtxoDetail{
			TxID:     record.TxHash,
			Index:    index,
			Height:   record.Height,
			Satoshis: satoshis,
		})
	}
	return details
}

// scriptRecordToHistoryItem maps an SDK script record to a wdk script history item.
// Confirmed records carry their block height; unconfirmed records have a nil height.
func scriptRecordToHistoryItem(record *wocsdk.ScriptRecord, confirmed bool) wdk.ScriptHistoryItem {
	item := wdk.ScriptHistoryItem{TxHash: record.TxHash}
	if confirmed {
		height := int(record.Height)
		item.Height = &height
	}
	return item
}

// validateScriptHash validates that a script hash is a non-empty hex string of a plausible length.
func validateScriptHash(scriptHash string) error {
	if scriptHash == "" {
		return fmt.Errorf("scripthash cannot be empty")
	}

	if len(scriptHash) < 20 {
		return fmt.Errorf("invalid scripthash length: too short (minimum 20 characters)")
	}

	if len(scriptHash) > 66 {
		return fmt.Errorf("invalid scripthash length: too long (maximum 66 characters)")
	}

	if _, err := hex.DecodeString(scriptHash); err != nil {
		return fmt.Errorf("invalid scripthash format: %w", err)
	}

	return nil
}

// chunkTxIDs splits txIDs into chunks of at most size elements.
func chunkTxIDs(txIDs []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	chunks := make([][]string, 0, (len(txIDs)+size-1)/size)
	for start := 0; start < len(txIDs); start += size {
		end := start + size
		if end > len(txIDs) {
			end = len(txIDs)
		}
		chunks = append(chunks, txIDs[start:end])
	}
	return chunks
}
