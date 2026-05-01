package funder

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"

	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/errfunder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var changeOutputSize = txutils.P2PKHOutputSize

const (
	utxoBatchSize = 1000
)

type UTXORepository interface {
	FindNotReservedUTXOs(
		ctx context.Context,
		userID int,
		basketName string,
		page *queryopts.Paging,
		forbiddenOutputIDs []uint,
		includeSending bool,
	) ([]*models.UserUTXO, error)
	CountUTXOs(ctx context.Context, userID int, basketName string) (int64, error)
}

type SQL struct {
	logger                *slog.Logger
	utxoRepository        UTXORepository
	feeCalculator         *feeCalc
	maxChangeOutputsPerTx atomic.Uint64
}

// NewSQL creates a new SQL funder. maxChangeOutputsPerTx limits how many change outputs are created
// per transaction; it can be updated at runtime via SetMaxChangeOutputsPerTx.
//
// Without this cap the wallet would attempt to create numberOfDesiredUTXOs change outputs in a
// single transaction, producing a very large transaction whose raw bytes are embedded in the BEEF
// of every subsequent child transaction. With the cap the UTXO pool builds gradually.
func NewSQL(logger *slog.Logger, utxoRepository UTXORepository, feeModel defs.FeeModel, maxChangeOutputsPerTx uint64) *SQL {
	logger = logging.Child(logger, "funderSQL")
	feeCalculator := newFeeCalculator(feeModel)

	s := &SQL{
		logger:         logger,
		utxoRepository: utxoRepository,
		feeCalculator:  feeCalculator,
	}
	s.maxChangeOutputsPerTx.Store(maxChangeOutputsPerTx)
	return s
}

// SetMaxChangeOutputsPerTx updates the per-transaction change output cap at runtime.
// Takes effect on the next Fund() call.
func (f *SQL) SetMaxChangeOutputsPerTx(n uint64) {
	f.maxChangeOutputsPerTx.Store(n)
}

func (f *SQL) Fund(
	ctx context.Context,
	targetSat satoshi.Value,
	currentTxSize uint64,
	outputCount uint64,
	basket *entity.OutputBasket,
	userID int,
	forbiddenOutputIDs []uint,
	priorityOutputs []*entity.Output,
	includeSending bool,
	isSweep bool,
) (*Result, error) {
	existing, err := f.utxoRepository.CountUTXOs(ctx, userID, basket.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate desired utxo number in basket: %w", err)
	}

	collector, err := newCollector(targetSat, currentTxSize, outputCount, basket.NumberOfDesiredUTXOs-existing, basket.MinimumDesiredUTXOValue, f.feeCalculator, f.maxChangeOutputsPerTx.Load(), isSweep)
	if err != nil {
		return nil, fmt.Errorf("failed to start collecting utxo: %w", err)
	}

	// Phase 1: Allocate priority outputs (noSend change from prior TXs in same batch).
	if err = f.allocatePriorityOutputs(collector, priorityOutputs, forbiddenOutputIDs, isSweep); err != nil {
		return nil, err
	}
	if !isSweep && collector.IsFunded() {
		return collector.GetResult()
	}

	// Phase 2: Load all eligible UTXOs into a tiered pool.
	pool, err := f.loadUTXOPool(ctx, userID, basket.Name, forbiddenOutputIDs, includeSending)
	if err != nil {
		return nil, err
	}

	// Phase 3: Sweep mode — allocate everything.
	if isSweep {
		return f.allocateSweep(collector, pool)
	}

	// Phase 4: Tiered best-fit selection.
	// Each round: compute remaining need, pick optimal UTXO (exact → best-fit → largest),
	// allocate it (which recalculates fee), repeat until funded.
	for !collector.IsFunded() {
		remaining := collector.remaining()
		if remaining <= 0 {
			break
		}

		utxo := pool.selectBest(uint64(remaining))
		if utxo == nil {
			return nil, errfunder.ErrNotEnoughFunds
		}
		if err = collector.allocateUTXO(utxo); err != nil {
			return nil, fmt.Errorf("failed to allocate utxo: %w", err)
		}
	}

	return collector.GetResult()
}

func (f *SQL) allocateSweep(collector *utxoCollector, pool *utxoPool) (*Result, error) {
	for _, utxo := range pool.all() {
		if err := collector.allocateUTXO(utxo); err != nil {
			return nil, fmt.Errorf("failed to allocate utxo in sweep: %w", err)
		}
	}
	return collector.GetResult()
}

func (f *SQL) allocatePriorityOutputs(collector *utxoCollector, priorityOutputs []*entity.Output, forbiddenOutputIDs []uint, isSweep bool) error {
	forbiddenIDs := make(map[uint]struct{}, len(forbiddenOutputIDs))
	for _, id := range forbiddenOutputIDs {
		forbiddenIDs[id] = struct{}{}
	}

	for _, output := range priorityOutputs {
		if _, ok := forbiddenIDs[output.ID]; ok {
			continue
		}

		sats, err := to.UInt64(output.Satoshis)
		if err != nil {
			return fmt.Errorf("failed to convert output satoshis: %d to uint64: %w", output.Satoshis, err)
		}

		var basket string
		if output.BasketName != nil {
			basket = *output.BasketName
		}

		utxo := &models.UserUTXO{
			UserID:             output.UserID,
			OutputID:           output.ID,
			BasketName:         basket,
			Satoshis:           sats,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
			CreatedAt:          output.CreatedAt,
		}

		if err = collector.allocateUTXO(utxo); err != nil {
			return fmt.Errorf("failed to allocate priority output: %w", err)
		}

		if !isSweep && collector.IsFunded() {
			return nil
		}
	}
	return nil
}

func (f *SQL) loadUTXOPool(ctx context.Context, userID int, basketName string, forbiddenOutputIDs []uint, includeSending bool) (*utxoPool, error) {
	// Load all eligible UTXOs. The repository sorts by status tier + satoshis ASC,
	// but the pool re-sorts internally per tier for 3-stage selection.
	page := &queryopts.Paging{
		Limit:  utxoBatchSize,
		SortBy: "satoshis",
	}

	var allUTXOs []*models.UserUTXO
	for {
		utxos, err := f.utxoRepository.FindNotReservedUTXOs(ctx, userID, basketName, page, forbiddenOutputIDs, includeSending)
		if err != nil {
			return nil, fmt.Errorf("failed to load utxos: %w", err)
		}

		allUTXOs = append(allUTXOs, utxos...)
		if len(utxos) < utxoBatchSize {
			break
		}
		page.Next()
	}

	return newUTXOPool(allUTXOs), nil
}

type utxoCollector struct {
	txSats satoshi.Value
	txSize uint64

	fee           satoshi.Value
	feeCalculator *feeCalc

	satsCovered    satoshi.Value
	allocatedUTXOs []*UTXO

	outputCount             uint64
	numberOfDesiredUTXOs    uint64
	minimumDesiredUTXOValue uint64
	maxChangeOutputsPerTx   uint64
	changeOutputsCount      uint64
	minimumChange           uint64
	// dustFloor is the minimum satoshi value a change output must have to be economically viable.
	// An output below this threshold costs more to spend in a future transaction than it is worth.
	dustFloor satoshi.Value
	isSweep   bool
}

func newCollector(txSats satoshi.Value, txSize, outputCount uint64, numberOfDesiredUTXOs int64, minimumDesiredUTXOValue uint64, feeCalculator *feeCalc, maxChangeOutputsPerTx uint64, isSweep bool) (c *utxoCollector, err error) {
	c = &utxoCollector{
		txSats:                  txSats,
		outputCount:             outputCount,
		minimumDesiredUTXOValue: minimumDesiredUTXOValue,
		maxChangeOutputsPerTx:   maxChangeOutputsPerTx,
		feeCalculator:           feeCalculator,
		allocatedUTXOs:          make([]*UTXO, 0),
		isSweep:                 isSweep,
	}

	err = c.increaseSize(txSize)
	if err != nil {
		return nil, fmt.Errorf("failed to increase transaction size: %w", err)
	}

	c.numberOfDesiredUTXOs = must.ConvertToUInt64(to.NoLessThan(numberOfDesiredUTXOs, 1))

	// Calculate dust floor: the minimum satoshi value for a change output to be worth spending.
	// We model the smallest possible future spend (1 P2PKH input + 1 P2PKH output)
	// and require each output to be worth at least 2× that future fee.
	// The absolute floor of 1 prevents nonsensical behavior at fee rate 0.
	minSpendTxSize := txutils.TransactionSizeFromScriptLengths(
		[]uint64{txutils.P2PKHUnlockingScriptLength},
		[]uint64{txutils.P2PKHLockingScriptLength},
	)
	c.dustFloor = satoshi.Value(math.Max(1, math.Ceil(float64(minSpendTxSize)/1000*feeCalculator.value)*2))

	c.calculateMinimumChange()

	err = c.calculateChangeOutputs()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate change outputs: %w", err)
	}

	return c, nil
}

func (c *utxoCollector) remaining() satoshi.Value {
	return satoshi.MustSubtract(c.satsToCover(), c.satsCovered)
}

func (c *utxoCollector) IsFunded() bool {
	// A valid Bitcoin transaction must have at least one output.
	// If no outputs are defined and no change outputs will be created,
	// we must continue allocating UTXOs to ensure at least one change output exists.
	totalOutputs := c.outputCount + c.changeOutputsCount
	if totalOutputs == 0 {
		return c.satsCovered > c.satsToCover()
	}

	if c.isSweep {
		return false
	}

	return c.satsCovered >= c.satsToCover()
}

func (c *utxoCollector) GetResult() (*Result, error) {
	if c.IsFunded() || c.isSweep {
		return c.prepareResult()
	}
	return nil, errfunder.ErrNotEnoughFunds
}

func (c *utxoCollector) allocateUTXO(utxo *models.UserUTXO) (err error) {
	c.addToAllocated(utxo)

	err = c.increaseSize(utxo.EstimatedInputSize)
	if err != nil {
		return fmt.Errorf("failed to increase tx size: %w", err)
	}

	err = c.increaseValue(satoshi.MustFrom(utxo.Satoshis))
	if err != nil {
		return fmt.Errorf("failed to increase tx value: %w", err)
	}

	err = c.calculateChangeOutputs()
	if err != nil {
		return fmt.Errorf("failed to calculate change outputs: %w", err)
	}

	return nil
}

func (c *utxoCollector) addToAllocated(utxo *models.UserUTXO) {
	c.allocatedUTXOs = append(c.allocatedUTXOs, &UTXO{
		OutputID: utxo.OutputID,
		Satoshis: satoshi.MustFrom(utxo.Satoshis),
	})
}

func (c *utxoCollector) increaseSize(size uint64) (err error) {
	c.txSize += size
	c.fee, err = c.feeCalculator.Calculate(c.txSize)
	if err != nil {
		return fmt.Errorf("failed to calculate fee: %w", err)
	}
	return nil
}

func (c *utxoCollector) increaseValue(sats satoshi.Value) error {
	var err error
	c.satsCovered, err = satoshi.Add(c.satsCovered, sats)
	if err != nil {
		return fmt.Errorf("cannot increase tx value: %w", err)
	}
	return nil
}

func (c *utxoCollector) satsToCover() satoshi.Value {
	return satoshi.MustAdd(c.txSats, c.fee)
}

func (c *utxoCollector) change() satoshi.Value {
	return satoshi.MustSubtract(c.satsCovered, c.satsToCover())
}

func (c *utxoCollector) prepareResult() (*Result, error) {
	changeAmount := c.change()

	// If the change amount is below the dust floor, it is uneconomical to create any change output.
	// Discard all change outputs and give the amount as extra fee to the miner.
	if changeAmount < c.dustFloor {
		c.changeOutputsCount = 0
	}

	return &Result{
		AllocatedUTXOs:     c.allocatedUTXOs,
		Fee:                c.fee,
		ChangeAmount:       changeAmount,
		ChangeOutputsCount: c.changeOutputsCount,
		DustFloor:          c.dustFloor,
	}, nil
}

func (c *utxoCollector) calculateChangeOutputs() error {
	change := c.change()
	if change <= 0 {
		return nil
	}

	c.calculateChangeCount(must.ConvertToUInt64(change))

	err := c.increaseSize(c.changeOutputsCount * changeOutputSize)
	if err != nil {
		return fmt.Errorf("failed to increase transaction size: %w", err)
	}

	return nil
}

func (c *utxoCollector) calculateChangeCount(changeVal uint64) {
	c.changeOutputsCount = changeVal/c.minimumDesiredUTXOValue + 1

	if changeVal%c.minimumDesiredUTXOValue < c.minimumChange {
		c.changeOutputsCount -= 1
	}

	capCount := c.numberOfDesiredUTXOs
	if c.maxChangeOutputsPerTx < capCount {
		capCount = c.maxChangeOutputsPerTx
	}

	c.changeOutputsCount = to.ValueBetween(c.changeOutputsCount, 1, capCount)

	dustFloorU64 := c.dustFloor.MustUInt64()
	for c.changeOutputsCount > 1 && changeVal/c.changeOutputsCount < dustFloorU64 {
		c.changeOutputsCount--
	}
}

// calculateMinimumChange determines the minimum change amount based on the **Desired** minimum UTXO value.
// The "desired" minimum UTXO value represents the user's preference for common UTXO values in the basket.
// In contrast, the minimum change is the threshold below which a new UTXO is not created.
func (c *utxoCollector) calculateMinimumChange() {
	c.minimumChange = c.minimumDesiredUTXOValue / 4
}
