package funder

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/errfunder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/to"
)

var changeOutputSize = txutils.P2PKHOutputSize

const utxoBatchSize = 1000

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
	logger         *slog.Logger
	utxoRepository UTXORepository
	feeCalculator  *feeCalc
}

func NewSQL(logger *slog.Logger, utxoRepository UTXORepository, feeModel defs.FeeModel) *SQL {
	logger = logging.Child(logger, "funderSQL")
	feeCalculator := newFeeCalculator(feeModel)

	return &SQL{
		logger:         logger,
		utxoRepository: utxoRepository,
		feeCalculator:  feeCalculator,
	}
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
) (*Result, error) {
	existing, err := f.utxoRepository.CountUTXOs(ctx, userID, basket.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate desired utxo number in basket: %w", err)
	}

	collector, err := newCollector(targetSat, currentTxSize, outputCount, basket.NumberOfDesiredUTXOs-existing, basket.MinimumDesiredUTXOValue, f.feeCalculator)
	if err != nil {
		return nil, fmt.Errorf("failed to start collecting utxo: %w", err)
	}

	utxos := f.loadUTXOs(ctx, userID, basket.Name, forbiddenOutputIDs, priorityOutputs, includeSending)

	err = collector.Allocate(utxos)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate utxos: %w", err)
	}

	return collector.GetResult()
}

func (f *SQL) loadUTXOs(ctx context.Context, userID int, basketName string, forbiddenOutputIDs []uint, priorityOutputs []*entity.Output, includeSending bool) iter.Seq2[*models.UserUTXO, error] {
	batches := seqerr.ProduceWithArg(
		func(page *queryopts.Paging) ([]*models.UserUTXO, *queryopts.Paging, error) {
			utxos, err := f.utxoRepository.FindNotReservedUTXOs(ctx, userID, basketName, page, forbiddenOutputIDs, includeSending)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load utxos: %w", err)
			}
			page.Next()
			return utxos, page, nil
		},
		&queryopts.Paging{
			Limit:  utxoBatchSize,
			SortBy: "satoshis",
		})

	standardUTXOs := seqerr.FlattenSlices(batches)
	if len(priorityOutputs) == 0 {
		return seqerr.Concat(standardUTXOs)
	}

	return seqerr.Concat(noSendChangeOutputsIterator(forbiddenOutputIDs, priorityOutputs), standardUTXOs)
}

func noSendChangeOutputsIterator(forbiddenOutputIDs []uint, priorityOutputs []*entity.Output) iter.Seq2[*models.UserUTXO, error] {
	forbiddenIDsLookup := make(map[uint]struct{}, len(forbiddenOutputIDs))
	for _, id := range forbiddenOutputIDs {
		forbiddenIDsLookup[id] = struct{}{}
	}

	return func(yield func(*models.UserUTXO, error) bool) {
		for _, output := range priorityOutputs {
			if _, ok := forbiddenIDsLookup[output.ID]; ok {
				continue
			}

			userID := output.UserID

			var basket string
			if output.BasketName != nil {
				basket = *output.BasketName
			}

			satoshis, err := to.UInt64(output.Satoshis)
			if err != nil {
				yield(nil, fmt.Errorf("failed to convert output satoshis: %d to uint64: %w", output.Satoshis, err))
				break
			}

			if !yield(&models.UserUTXO{
				UserID:             userID,
				OutputID:           output.ID,
				BasketName:         basket,
				Satoshis:           satoshis,
				EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
				CreatedAt:          output.CreatedAt,
			}, nil) {
				break
			}
		}
	}
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
	changeOutputsCount      uint64
	minimumChange           uint64
}

func newCollector(txSats satoshi.Value, txSize uint64, outputCount uint64, numberOfDesiredUTXOs int64, minimumDesiredUTXOValue uint64, feeCalculator *feeCalc) (c *utxoCollector, err error) {
	c = &utxoCollector{
		txSats:                  txSats,
		outputCount:             outputCount,
		minimumDesiredUTXOValue: minimumDesiredUTXOValue,
		feeCalculator:           feeCalculator,
		allocatedUTXOs:          make([]*UTXO, 0),
	}

	err = c.increaseSize(txSize)
	if err != nil {
		return nil, fmt.Errorf("failed to increase transaction size: %w", err)
	}

	c.numberOfDesiredUTXOs = must.ConvertToUInt64(to.NoLessThan(numberOfDesiredUTXOs, 1))

	c.calculateMinimumChange()

	err = c.calculateChangeOutputs()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate change outputs: %w", err)
	}

	return c, nil
}

func (c *utxoCollector) Allocate(utxos iter.Seq2[*models.UserUTXO, error]) error {
	utxos = seqerr.TakeUntilTrue(utxos, c.IsFunded)
	err := seqerr.ForEach(utxos, c.allocateUTXO)
	if err != nil {
		return fmt.Errorf("failed to allocate utxo: %w", err)
	}
	return nil
}

func (c *utxoCollector) IsFunded() bool {
	// A valid Bitcoin transaction must have at least one output.
	// If no outputs are defined and no change outputs will be created,
	// we must continue allocating UTXOs to ensure at least one change output exists.
	totalOutputs := c.outputCount + c.changeOutputsCount
	if totalOutputs == 0 {
		return c.satsCovered > c.satsToCover()
	}

	return c.satsCovered >= c.satsToCover()
}

func (c *utxoCollector) GetResult() (*Result, error) {
	if c.IsFunded() {
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

	// If adding a change output increases the fee to the point where no change remains,
	// the change outputs are discarded, and the additional amount is given as a higher fee to the miner.
	if changeAmount == 0 {
		c.changeOutputsCount = 0
	}

	return &Result{
		AllocatedUTXOs:     c.allocatedUTXOs,
		Fee:                c.fee,
		ChangeAmount:       changeAmount,
		ChangeOutputsCount: c.changeOutputsCount,
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

	c.changeOutputsCount = to.ValueBetween(c.changeOutputsCount, 1, c.numberOfDesiredUTXOs)
}

// calculateMinimumChange determines the minimum change amount based on the **Desired** minimum UTXO value.
// The "desired" minimum UTXO value represents the user's preference for common UTXO values in the basket.
// In contrast, the minimum change is the threshold below which a new UTXO is not created.
func (c *utxoCollector) calculateMinimumChange() {
	c.minimumChange = c.minimumDesiredUTXOValue / 4
}
