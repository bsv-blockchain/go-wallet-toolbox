package testabilities

import (
	"encoding/json"
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncAssertion interface {
	Chunk(chunk *wdk.SyncChunk) SyncChunkAssertion
	DBState(storage StorageReader) DBStateAssertion
}

type SyncChunkAssertion interface {
	WithoutError(err error) ValidSyncChunkAssertion
	WithError(error)
}

type ValidSyncChunkAssertion interface {
	WithFromStorageIdentityKey(key string) ValidSyncChunkAssertion
	WithToStorageIdentityKey(key string) ValidSyncChunkAssertion
	WithUserIdentityKey(key string) ValidSyncChunkAssertion
	WithUser(userIdentityKey, storageIdentityKey string) ValidSyncChunkAssertion
	WithoutUser() ValidSyncChunkAssertion
	WithGeneralInfo(info *wdk.RequestSyncChunkArgs) ValidSyncChunkAssertion

	AllCountZero() ValidSyncChunkAssertion

	BasketsCount(length int) ValidSyncChunkAssertion
	BasketAtIndex(index int) BasketAssertion

	ProvenTxReqsCount(length int) ValidSyncChunkAssertion
	ProvenTxReqAtIndex(index int) ProvenTxReqAssertion

	ProvenTxsCount(length int) ValidSyncChunkAssertion
	ProvenTxAtIndex(index int) ProvenTxAssertion

	TransactionsCount(length int) ValidSyncChunkAssertion
	TransactionAtIndex(index int) TransactionAssertion

	OutputsCount(length int) ValidSyncChunkAssertion
	OutputAtIndex(index int) OutputAssertion

	LabelsCount(length int) ValidSyncChunkAssertion
	LabelsMapCount(length int) ValidSyncChunkAssertion
	WithTxLabels(transactionID uint, labels ...string) ValidSyncChunkAssertion

	TagsCount(length int) ValidSyncChunkAssertion
	TagsMapCount(length int) ValidSyncChunkAssertion
	WithOutputTag(outputID uint, tags ...string) ValidSyncChunkAssertion
}

type BasketAssertion interface {
	WithUserID(userID int) BasketAssertion
	HasValidID() BasketAssertion
	IsDefaultBasket() BasketAssertion
}

type ProvenTxReqAssertion interface {
	AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxReqAssertion
	WithTxID(txID string) ProvenTxReqAssertion
	HasHistoryNotes(whatFields ...string) ProvenTxReqAssertion
}

type ProvenTxAssertion interface {
	AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxAssertion
	HasMerklePath() ProvenTxAssertion
}

type TransactionAssertion interface {
	WithTxID(txID string) TransactionAssertion
	WithoutTxID() TransactionAssertion
	WithProvenTxID(provenTxID int) TransactionAssertion
	WithoutProvenTxID() TransactionAssertion
	WithReference(reference string) TransactionAssertion
}

type OutputAssertion interface {
	WithTransactionID(transactionNumID uint) OutputAssertion
	WithoutBasketID() OutputAssertion
	WithBasketID(basketID int) OutputAssertion
}

type syncAssertion struct {
	testing.TB
}

func ThenSync(t testing.TB) SyncAssertion {
	t.Helper()
	return &syncAssertion{
		TB: t,
	}
}

func (s *syncAssertion) Chunk(chunk *wdk.SyncChunk) SyncChunkAssertion {
	s.Helper()
	return &syncChunkAssertion{
		TB:    s.TB,
		chunk: chunk,
	}
}

func (s *syncAssertion) DBState(storage StorageReader) DBStateAssertion {
	s.Helper()
	require.NotNil(s, storage, "Expected storage to be not nil")

	return ThenDBState(s, storage)
}

func (s *syncChunkAssertion) WithoutError(err error) ValidSyncChunkAssertion {
	s.Helper()
	assert.NotNil(s, s.chunk, "Expected chunk to be not nil")
	require.NoError(s, err, "Expected no error but got one")
	return s
}

func (s *syncChunkAssertion) WithError(err error) {
	s.Helper()
	require.Error(s, err, "Expected an error but got nil")
}

type syncChunkAssertion struct {
	testing.TB

	chunk *wdk.SyncChunk
}

func (s *syncChunkAssertion) WithFromStorageIdentityKey(key string) ValidSyncChunkAssertion {
	s.Helper()
	assert.Equal(s, key, s.chunk.FromStorageIdentityKey)
	return s
}

func (s *syncChunkAssertion) WithToStorageIdentityKey(key string) ValidSyncChunkAssertion {
	s.Helper()
	assert.Equal(s, key, s.chunk.ToStorageIdentityKey)
	return s
}

func (s *syncChunkAssertion) WithUserIdentityKey(key string) ValidSyncChunkAssertion {
	s.Helper()
	assert.Equal(s, key, s.chunk.UserIdentityKey)
	return s
}

func (s *syncChunkAssertion) WithUser(userIdentityKey, storageIdentityKey string) ValidSyncChunkAssertion {
	s.Helper()
	require.NotNil(s, s.chunk.User)
	assert.Equal(s, userIdentityKey, s.chunk.User.IdentityKey)
	assert.Equal(s, storageIdentityKey, s.chunk.User.ActiveStorage)
	return s
}

func (s *syncChunkAssertion) WithoutUser() ValidSyncChunkAssertion {
	s.Helper()
	require.Nil(s, s.chunk.User, "Expected chunk to have no user")
	return s
}

func (s *syncChunkAssertion) AllCountZero() ValidSyncChunkAssertion {
	s.Helper()
	s.BasketsCount(0)
	s.ProvenTxReqsCount(0)
	s.ProvenTxsCount(0)
	s.TransactionsCount(0)
	s.LabelsCount(0)
	s.LabelsMapCount(0)
	s.TagsCount(0)
	s.TagsMapCount(0)
	return s
}

type ChunkGeneralInfo struct {
	UserIdentityKey        string
	FromStorageIdentityKey string
	ToStorageIdentityKey   string
}

func (s *syncChunkAssertion) WithGeneralInfo(args *wdk.RequestSyncChunkArgs) ValidSyncChunkAssertion {
	s.Helper()
	s.WithFromStorageIdentityKey(args.FromStorageIdentityKey)
	s.WithToStorageIdentityKey(args.ToStorageIdentityKey)
	s.WithUserIdentityKey(args.IdentityKey)
	s.WithUser(args.IdentityKey, args.FromStorageIdentityKey)
	return s
}

func (s *syncChunkAssertion) BasketsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	require.Len(s, s.chunk.OutputBaskets, length)
	return s
}

func (s *syncChunkAssertion) BasketAtIndex(index int) BasketAssertion {
	s.Helper()
	require.GreaterOrEqual(s, index, 0)
	require.Less(s, index, len(s.chunk.OutputBaskets))
	basket := s.chunk.OutputBaskets[index]
	return &basketAssertion{
		parent: s,
		basket: basket,
	}
}

type basketAssertion struct {
	parent *syncChunkAssertion
	basket *wdk.TableOutputBasket
}

func (b *basketAssertion) WithUserID(userID int) BasketAssertion {
	b.parent.Helper()
	assert.Equal(b.parent, userID, b.basket.UserID, "Expected basket to have the same user ID as the test user")
	return b
}

func (b *basketAssertion) HasValidID() BasketAssertion {
	b.parent.Helper()
	assert.Positive(b.parent, b.basket.BasketID, "Expected basket to have a valid ID")
	return b
}

func (b *basketAssertion) IsDefaultBasket() BasketAssertion {
	b.parent.Helper()
	assert.Equal(b.parent, wdk.DefaultBasketConfiguration(), b.basket.BasketConfiguration, "Expected basket to have default configuration")
	return b
}

func (s *syncChunkAssertion) ProvenTxReqsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.ProvenTxReqs, length)
	return s
}

func (s *syncChunkAssertion) ProvenTxReqAtIndex(index int) ProvenTxReqAssertion {
	s.Helper()
	require.GreaterOrEqual(s, index, 0)
	require.Less(s, index, len(s.chunk.ProvenTxReqs))
	txReq := s.chunk.ProvenTxReqs[index]
	require.NotNil(s, txReq, "Expected txReq to be not nil")
	return &proveTxReqAssertion{
		parent: s,
		txReq:  txReq,
	}
}

type proveTxReqAssertion struct {
	parent *syncChunkAssertion
	txReq  *wdk.TableProvenTxReq
}

func (p *proveTxReqAssertion) AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxReqAssertion {
	p.parent.Helper()
	assert.Equal(p.parent, txSpec.ID().String(), p.txReq.TxID, "Expected txReq to align with transaction spec TxID")
	assert.Equal(p.parent, txSpec.TX().Bytes(), []byte(p.txReq.RawTx), "Expected txReq to align with transaction spec RawTx")
	return p
}

func (p *proveTxReqAssertion) HasHistoryNotes(whatFields ...string) ProvenTxReqAssertion {
	p.parent.Helper()

	historyNotes := p.getHistoryNotes()

	assert.Len(p.parent, historyNotes, len(whatFields), "Expected txReq to have %d history notes", len(whatFields))

	actualWhatFields := slices.Map(historyNotes, func(note map[string]any) string {
		what, ok := note["what"].(string)
		require.True(p.parent, ok, "Expected 'what' field to be a string in history notes")
		return what
	})
	assert.ElementsMatch(p.parent, whatFields, actualWhatFields, "Expected history notes to contain specific 'what' fields")
	return p
}

func (p *proveTxReqAssertion) getHistoryNotes() []map[string]any {
	p.parent.Helper()
	var notesObj struct {
		Notes []map[string]any `json:"notes"`
	}
	err := json.Unmarshal([]byte(p.txReq.History), &notesObj)
	require.NoError(p.parent, err, "Expected to unmarshal history notes without error")

	return notesObj.Notes
}

func (p *proveTxReqAssertion) WithTxID(txID string) ProvenTxReqAssertion {
	p.parent.Helper()
	assert.Equal(p.parent, txID, p.txReq.TxID)
	return p
}

func (s *syncChunkAssertion) ProvenTxsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.ProvenTxs, length)
	return s
}

func (s *syncChunkAssertion) ProvenTxAtIndex(index int) ProvenTxAssertion {
	s.Helper()
	require.GreaterOrEqual(s, index, 0)
	require.Less(s, index, len(s.chunk.ProvenTxs))
	tx := s.chunk.ProvenTxs[index]
	require.NotNil(s, tx, "Expected tx to be not nil")
	return &proveTxAssertion{
		parent: s,
		tx:     tx,
	}
}

type proveTxAssertion struct {
	parent *syncChunkAssertion
	tx     *wdk.TableProvenTx
}

func (p *proveTxAssertion) AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxAssertion {
	p.parent.Helper()
	assert.Equal(p.parent, txSpec.ID().String(), p.tx.TxID, "Expected tx to align with transaction spec TxID")
	assert.Equal(p.parent, txSpec.TX().Bytes(), []byte(p.tx.RawTx), "Expected tx to align with transaction spec RawTx")
	return p
}

func (p *proveTxAssertion) HasMerklePath() ProvenTxAssertion {
	p.parent.Helper()
	assert.NotEmpty(p.parent, p.tx.MerklePath, "Expected tx to have a non-empty MerklePath")
	return p
}

func (s *syncChunkAssertion) TransactionsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.Transactions, length, "Expected chunk to have %d transactions", length)
	return s
}

func (s *syncChunkAssertion) TransactionAtIndex(index int) TransactionAssertion {
	s.Helper()
	require.GreaterOrEqual(s, index, 0)
	require.Less(s, index, len(s.chunk.Transactions))
	tx := s.chunk.Transactions[index]
	require.NotNil(s, tx, "Expected transaction to be not nil")
	return &transactionAssertion{
		parent: s,
		tx:     tx,
	}
}

type transactionAssertion struct {
	parent *syncChunkAssertion
	tx     *wdk.TableTransaction
}

func (t *transactionAssertion) WithTxID(txID string) TransactionAssertion {
	t.parent.Helper()
	if !assert.NotNil(t.parent, t.tx.TxID) {
		return t
	}
	assert.Equal(t.parent, txID, *t.tx.TxID)
	return t
}

func (t *transactionAssertion) WithoutTxID() TransactionAssertion {
	t.parent.Helper()
	assert.Nil(t.parent, t.tx.TxID)
	return t
}

func (t *transactionAssertion) WithProvenTxID(provenTxID int) TransactionAssertion {
	t.parent.Helper()
	if !assert.NotNil(t.parent, t.tx.ProvenTxID) {
		return t
	}
	assert.Equal(t.parent, provenTxID, *t.tx.ProvenTxID)
	return t
}

func (t *transactionAssertion) WithoutProvenTxID() TransactionAssertion {
	t.parent.Helper()
	assert.Nil(t.parent, t.tx.ProvenTxID)
	return t
}

func (t *transactionAssertion) WithReference(reference string) TransactionAssertion {
	t.parent.Helper()
	assert.Equal(t.parent, reference, string(t.tx.Reference))
	return t
}

func (s *syncChunkAssertion) OutputsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.Outputs, length, "Expected chunk to have %d outputs", length)
	return s
}

func (s *syncChunkAssertion) OutputAtIndex(index int) OutputAssertion {
	s.Helper()
	require.GreaterOrEqual(s, index, 0)
	require.Less(s, index, len(s.chunk.Outputs))
	output := s.chunk.Outputs[index]
	require.NotNil(s, output, "Expected output to be not nil")
	return &outputAssertion{
		parent: s,
		output: output,
	}
}

type outputAssertion struct {
	parent *syncChunkAssertion
	output *wdk.TableOutput
}

func (o *outputAssertion) WithTransactionID(transactionNumID uint) OutputAssertion {
	o.parent.Helper()
	assert.Equal(o.parent, transactionNumID, o.output.TransactionID, "Expected output to have the same transaction ID as the one requested")
	return o
}

func (o *outputAssertion) WithoutBasketID() OutputAssertion {
	o.parent.Helper()
	assert.Nil(o.parent, o.output.BasketID, "Expected output to have no BasketID")
	return o
}

func (o *outputAssertion) WithBasketID(basketID int) OutputAssertion {
	o.parent.Helper()
	if !assert.NotNil(o.parent, o.output.BasketID) {
		return o
	}
	assert.Equal(o.parent, basketID, *o.output.BasketID, "Expected output to have the same BasketID as the one requested")
	return o
}

func (s *syncChunkAssertion) LabelsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.TxLabels, length, "Expected chunk to have %d labels", length)
	return s
}

func (s *syncChunkAssertion) WithTxLabels(transactionID uint, labels ...string) ValidSyncChunkAssertion {
	s.Helper()

	for _, label := range labels {
		s.withLabel(transactionID, label)
	}

	return s
}

func (s *syncChunkAssertion) LabelsMapCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.TxLabelMaps, length, "Expected chunk to have %d label maps", length)
	return s
}

func (s *syncChunkAssertion) withLabel(transactionID uint, label string) ValidSyncChunkAssertion {
	s.Helper()
	require.NotEmpty(s, s.chunk.TxLabels)
	require.NotEmpty(s, s.chunk.TxLabelMaps)

	labelLookup := make(map[uint]*wdk.TableTxLabel)
	for _, txLabel := range s.chunk.TxLabels {
		labelLookup[txLabel.TxLabelID] = txLabel
	}

	assert.Condition(s, func() bool {
		for _, connection := range s.chunk.TxLabelMaps {
			if connection.TransactionID == transactionID && labelLookup[connection.TxLabelID].Label == label {
				return true
			}
		}
		return false
	}, "Expected chunk to contain label '%s' for transaction ID %d", label, transactionID)
	return s
}

func (s *syncChunkAssertion) TagsCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.OutputTags, length, "Expected chunk to have %d tags", length)
	return s
}

func (s *syncChunkAssertion) TagsMapCount(length int) ValidSyncChunkAssertion {
	s.Helper()
	assert.Len(s, s.chunk.OutputTagMaps, length, "Expected chunk to have %d tag maps", length)
	return s
}

func (s *syncChunkAssertion) WithOutputTag(outputID uint, tags ...string) ValidSyncChunkAssertion {
	s.Helper()

	for _, label := range tags {
		s.withTag(outputID, label)
	}

	return s
}

func (s *syncChunkAssertion) withTag(outputID uint, tag string) ValidSyncChunkAssertion {
	s.Helper()
	require.NotEmpty(s, s.chunk.OutputTags)
	require.NotEmpty(s, s.chunk.OutputTagMaps)

	tagLookup := make(map[uint]*wdk.TableOutputTag)
	for _, outputTag := range s.chunk.OutputTags {
		tagLookup[outputTag.OutputTagID] = outputTag
	}

	assert.Condition(s, func() bool {
		for _, connection := range s.chunk.OutputTagMaps {
			if connection.OutputID == outputID && tagLookup[connection.OutputTagID].Tag == tag {
				return true
			}
		}
		return false
	}, "Expected chunk to contain tag %q for transaction ID %d", tag, outputID)
	return s
}
