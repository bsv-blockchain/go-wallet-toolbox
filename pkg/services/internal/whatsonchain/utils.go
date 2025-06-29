package whatsonchain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

func waitOrCancel(ctx context.Context, delay time.Duration, txid string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for tx %s: %w", txid, ctx.Err())
	case <-time.After(delay):
		return nil
	}
}

func classifyBroadcastStatus(status BroadcastStatus) (wdk.PostedTxIDResultStatus, []string) {
	switch status {
	case StatusSuccess:
		return wdk.PostedTxIDResultSuccess, nil
	case StatusAlreadyBroadcasted:
		return wdk.PostedTxIDResultAlreadyKnown, []string{"Transaction already in mempool"}
	case StatusDoubleSpend:
		return wdk.PostedTxIDResultDoubleSpend, []string{"Double spend detected"}
	case StatusMissingInputs:
		return wdk.PostedTxIDResultMissingInputs, []string{"Missing inputs detected"}
	case StatusError:
		return wdk.PostedTxIDResultError, []string{"Broadcast status error"}
	default:
		return wdk.PostedTxIDResultError, []string{fmt.Sprintf("Unknown error: unexpected BroadcastStatus value '%v'", status)}
	}
}

func containsI(subject string, contains ...string) bool {
	subject = strings.ToLower(subject)
	for _, c := range contains {
		if strings.Contains(subject, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

func firstNonNilError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func convertTscProofToMerklePath(txid string, index int, nodes []string, blockHeight uint32) (*transaction.MerklePath, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes provided in TSC proof for txid %s", txid)
	}

	txidHash, err := chainhash.NewHashFromHex(txid)
	if err != nil {
		return nil, fmt.Errorf("invalid txid: %w", err)
	}

	level0, nextIndex, err := buildLevel0PathElement(txid, txidHash, nodes[0], index)
	if err != nil {
		return nil, fmt.Errorf("failed to build level 0 path element: %w", err)
	}

	upperLevels, err := buildUpperLevels(nodes, 1, nextIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to build upper levels: %w", err)
	}

	treeHeight := len(nodes)
	path := make([][]*transaction.PathElement, treeHeight)
	path[0] = level0
	for i := 1; i < treeHeight; i++ {
		path[i] = upperLevels[i]
	}

	return transaction.NewMerklePath(blockHeight, path), nil
}

func buildLevel0PathElement(txid string, txidHash *chainhash.Hash, node string, index int) ([]*transaction.PathElement, int, error) {
	isOdd := index%2 == 1
	siblingIndex := index ^ 1

	sibling, err := createPathElement(node, siblingIndex, true, txid)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid node hash at level 0: %w", err)
	}

	offset, err := to.UInt64(index)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid index %d: %w", index, err)
	}
	txidLeaf := &transaction.PathElement{
		Offset: offset,
		Hash:   txidHash,
		Txid:   to.Ptr(true),
	}

	var level0 []*transaction.PathElement
	if isOdd {
		level0 = []*transaction.PathElement{sibling, txidLeaf}
	} else {
		level0 = []*transaction.PathElement{txidLeaf, sibling}
	}

	nextIndex := index >> 1
	return level0, nextIndex, nil
}

func buildUpperLevels(nodes []string, startLevel int, startIndex int) ([][]*transaction.PathElement, error) {
	treeHeight := len(nodes)
	path := make([][]*transaction.PathElement, treeHeight)

	currentIndex := startIndex

	for level := startLevel; level < treeHeight; level++ {
		siblingIndex := currentIndex ^ 1

		sibling, err := createPathElement(nodes[level], siblingIndex, false, "")
		if err != nil {
			return nil, fmt.Errorf("invalid node hash at level %d: %w", level, err)
		}

		path[level] = []*transaction.PathElement{sibling}
		currentIndex >>= 1
	}

	return path, nil
}

// createPathElement builds a PathElement given node string and sibling index.
func createPathElement(node string, siblingIndex int, isLevel0 bool, txid string) (*transaction.PathElement, error) {
	const duplicateNodeMarker = "*"

	offset, err := to.UInt64(siblingIndex)
	if err != nil {
		return nil, fmt.Errorf("invalid sibling index %d: %w", siblingIndex, err)
	}
	element := &transaction.PathElement{
		Offset: offset,
	}

	if node == duplicateNodeMarker || (isLevel0 && node == txid) {
		element.Duplicate = to.Ptr(true)
	} else {
		nodeHash, err := chainhash.NewHashFromHex(node)
		if err != nil {
			return nil, fmt.Errorf("invalid node hash %q: %w", node, err)
		}
		element.Hash = nodeHash
	}

	return element, nil
}
