package testabilities

import (
	"context"
	"maps"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StorageReader interface {
	FindKnownTx(ctx context.Context, txID string) (*entity.KnownTx, error)
}

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
}

type BasketAssertion interface {
	WithUserID(userID int) BasketAssertion
	HasValidID() BasketAssertion
	IsDefaultBasket() BasketAssertion
}

type ProvenTxReqAssertion interface {
	AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxReqAssertion
}

type ProvenTxAssertion interface {
	AlignsWithTxSpec(txSpec testvectors.TransactionSpec) ProvenTxAssertion
	HasMerklePath() ProvenTxAssertion
}

type DBStateAssertion interface {
	HasKnownTXs(txIDs ...string) DBStateAssertion
	HasKnownTX(txID string) KnownTxAssertion
}

type KnownTxAssertion interface {
	WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion
	IsMined() KnownTxAssertion
	HasRawTx() KnownTxAssertion
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

	return &dbStateAssertion{
		TB:      s.TB,
		storage: storage,
	}
}

func (s *syncChunkAssertion) WithoutError(err error) ValidSyncChunkAssertion {
	s.Helper()
	require.NotNil(s, s.chunk, "Expected chunk to be not nil")
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
	assert.True(b.parent, b.basket.BasketID > 0, "Expected basket to have a valid ID")
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
	assert.Equal(p.parent, txSpec.ID(), p.txReq.TxID, "Expected txReq to align with transaction spec TxID")
	assert.Equal(p.parent, txSpec.TX().Bytes(), []byte(p.txReq.RawTx), "Expected txReq to align with transaction spec RawTx")
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
	assert.Equal(p.parent, txSpec.ID(), p.tx.TxID, "Expected tx to align with transaction spec TxID")
	assert.Equal(p.parent, txSpec.TX().Bytes(), []byte(p.tx.RawTx), "Expected tx to align with transaction spec RawTx")
	return p
}

func (p *proveTxAssertion) HasMerklePath() ProvenTxAssertion {
	p.parent.Helper()
	assert.NotEmpty(p.parent, p.tx.MerklePath, "Expected tx to have a non-empty MerklePath")
	return p
}

type dbStateAssertion struct {
	testing.TB
	storage StorageReader
}

func (d *dbStateAssertion) HasKnownTXs(txIDs ...string) DBStateAssertion {
	d.Helper()

	missingTXs := map[string]struct{}{}

	for _, txID := range txIDs {
		knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
		require.NoError(d.TB, err, txID)

		if knownTx == nil {
			missingTXs[txID] = struct{}{}
		}
	}

	if len(missingTXs) != 0 {
		missingIDs := seq.Collect(maps.Keys(missingTXs))
		assert.Failf(d, "Expected to find all the transactions", "missing transaction IDs: %v", missingIDs)
	}

	return d
}

func (d *dbStateAssertion) HasKnownTX(txID string) KnownTxAssertion {
	d.Helper()

	knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
	require.NoError(d.TB, err, txID)

	if knownTx == nil {
		require.Failf(d, "Expected to find the transaction", "transaction ID: %s", txID)
		return nil
	}

	assert.Equal(d, txID, knownTx.TxID, "Expected known transaction to have the same TxID as the one requested")

	return &knownTxAssertion{
		TB:      d.TB,
		knownTx: knownTx,
	}
}

type knownTxAssertion struct {
	testing.TB
	knownTx *entity.KnownTx
}

func (d *knownTxAssertion) WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, state, d.knownTx.Status, "Expected known transaction to have the status %s", state)
	return d
}

func (d *knownTxAssertion) IsMined() KnownTxAssertion {
	d.Helper()
	assert.NotNil(d, d.knownTx.BlockHeight)
	assert.NotEmpty(d, d.knownTx.MerklePath)
	assert.NotEmpty(d, d.knownTx.MerkleRoot)
	assert.NotEmpty(d, d.knownTx.BlockHash)
	return d
}

func (d *knownTxAssertion) HasRawTx() KnownTxAssertion {
	d.Helper()
	assert.NotEmpty(d, d.knownTx.RawTx, "Expected known transaction to have a non-empty RawTx")
	return d
}
