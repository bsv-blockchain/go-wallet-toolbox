package bitails

import (
	"errors"
	"fmt"
	"net/url"
	"path"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

func classifyBroadcastStatus(err error) (alreadyKnown, doubleSpend bool, note string) {
	if err == nil {
		return false, false, ""
	}
	switch {
	case errors.Is(err, ErrAlreadyKnown):
		return true, false, "Transaction already in mempool"
	case errors.Is(err, ErrMissingInputs):
		return false, true, "Missing inputs (double spend)"
	default:
		return false, false, err.Error()
	}
}

// buildTxStatusURL constructs a full URL to fetch the status of a transaction.
func buildTxStatusURL(baseURL, txID string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	u.Path = path.Join(u.Path, "tx", txID, "status")
	return u.String(), nil
}

func convertBranchProofToMerklePath(txid string, branches []merkleBranch, blockHeight uint32) (*transaction.MerklePath, error) {
	txidHash, err := chainhash.NewHashFromHex(txid)
	if err != nil {
		return nil, fmt.Errorf("invalid txid: %w", err)
	}

	treeHeight := len(branches) + 1
	path := make([][]*transaction.PathElement, treeHeight)

	txidLeaf := &transaction.PathElement{
		Offset: 0,
		Hash:   txidHash,
		Txid:   to.Ptr(true),
	}
	path[0] = []*transaction.PathElement{txidLeaf}

	offset := uint64(0)

	for i, b := range branches {
		siblingHash, err := chainhash.NewHashFromHex(b.Hash)
		if err != nil {
			return nil, fmt.Errorf("invalid branch hash: %w", err)
		}

		offset = offset / 2
		if b.Pos == Left {
			offset = offset*2 + 1
		} else if b.Pos == Right {
			offset = offset * 2
		} else {
			return nil, fmt.Errorf("invalid branch position: %q", b.Pos)
		}

		path[i+1] = []*transaction.PathElement{{
			Offset:    offset,
			Hash:      siblingHash,
			Txid:      nil,
			Duplicate: nil,
		}}
	}

	return transaction.NewMerklePath(blockHeight, path), nil
}
