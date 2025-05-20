package actions

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/commission"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	derivationLength = 16
	referenceLength  = 12
)

type UTXO struct {
	OutputID uint
	Satoshis satoshi.Value
}

type FundingResult struct {
	AllocatedUTXOs []*UTXO
	ChangeCount    uint64
	ChangeAmount   satoshi.Value
	Fee            satoshi.Value
}

func (fr *FundingResult) TotalAllocated() (satoshi.Value, error) {
	total, err := satoshi.Sum(seq.Map(seq.FromSlice(fr.AllocatedUTXOs), func(utxo *UTXO) satoshi.Value {
		return utxo.Satoshis
	}))
	if err != nil {
		return 0, fmt.Errorf("failed to sum allocated UTXOs: %w", err)
	}

	return total, nil
}

type CreateActionParams struct {
	Version                  uint32
	LockTime                 uint32
	Description              string
	Labels                   []primitives.StringUnder300
	Outputs                  []wdk.ValidCreateActionOutput
	Inputs                   []wdk.ValidCreateActionInput
	RandomizeOutputs         bool
	IncludeInputSourceRawTxs bool
}

func FromValidCreateActionArgs(args *wdk.ValidCreateActionArgs) CreateActionParams {
	// TODO: use only the necessary fields (no redundant fields)
	return CreateActionParams{
		Version:                  args.Version,
		LockTime:                 args.LockTime,
		Description:              string(args.Description),
		Labels:                   args.Labels,
		Outputs:                  args.Outputs,
		Inputs:                   args.Inputs,
		RandomizeOutputs:         args.Options.RandomizeOutputs,
		IncludeInputSourceRawTxs: args.IsSignAction && args.IncludeAllSourceTransactions,
	}
}

type Funder interface {
	// Fund
	// @param targetSat - the target amount of satoshis to fund (total inputs - total outputs)
	// @param currentTxSize - the current size of the transaction in bytes (size of tx + current inputs + current outputs)
	// @param numberOfDesiredUTXOs - the number of UTXOs in basket #TakeFromBasket
	// @param minimumDesiredUTXOValue - the minimum value of UTXO in basket #TakeFromBasket
	// @param userID - the user ID
	Fund(ctx context.Context, targetSat satoshi.Value, currentTxSize uint64, basket *wdk.TableOutputBasket, userID int) (*FundingResult, error)
}

type create struct {
	logger        *slog.Logger
	funder        Funder
	basketRepo    BasketRepo
	txRepo        TransactionsRepo
	outputRepo    OutputRepo
	provenTxRepo  ProvenTxRepo
	commission    *commission.ScriptGenerator
	commissionCfg defs.Commission
	random        wdk.Randomizer
}

func newCreateAction(
	logger *slog.Logger,
	funder Funder,
	commissionCfg defs.Commission,
	basketRepo BasketRepo,
	txRepo TransactionsRepo,
	outputRepo OutputRepo,
	provenTxRepo ProvenTxRepo,
	random wdk.Randomizer,
) *create {
	logger = logging.Child(logger, "createAction")
	c := &create{
		logger:        logger,
		funder:        funder,
		basketRepo:    basketRepo,
		txRepo:        txRepo,
		commissionCfg: commissionCfg,
		outputRepo:    outputRepo,
		provenTxRepo:  provenTxRepo,
		random:        random,
	}

	if commissionCfg.Enabled() {
		c.commission = commission.NewScriptGenerator(string(commissionCfg.PubKeyHex))
	}

	return c
}

func (c *create) Create(ctx context.Context, userID int, params CreateActionParams) (*wdk.StorageCreateActionResult, error) {
	basket, err := c.basketRepo.FindBasketByName(ctx, userID, wdk.BasketNameForChange)
	if err != nil {
		return nil, fmt.Errorf("failed to find basket for change: %w", err)
	}
	if basket == nil {
		return nil, fmt.Errorf("basket for change (%s) not found", wdk.BasketNameForChange)
	}

	xoutputs := seq.PointersFromSlice(params.Outputs)
	xinputs := seq.PointersFromSlice(params.Inputs)

	var commOut *serviceChargeOutput
	if c.commission != nil {
		commOut, err = c.createCommissionOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to collect outputs: %w", err)
		}
		xoutputs = seq.Append(xoutputs, &commOut.ValidCreateActionOutput)
	}

	initialTxSize, err := c.txSize(xinputs, xoutputs)
	if err != nil {
		return nil, err
	}

	targetSat, err := c.targetSat(xinputs, xoutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target satoshis: %w", err)
	}

	funding, err := c.funder.Fund(ctx, targetSat, initialTxSize, basket, userID)
	if err != nil {
		return nil, fmt.Errorf("funding failed: %w", err)
	}

	changeDistribution := txutils.NewChangeDistribution(satoshi.MustFrom(basket.MinimumDesiredUTXOValue), c.random.Uint64).
		Distribute(funding.ChangeCount, funding.ChangeAmount)

	derivationPrefix, reference, err := c.randomValues()
	if err != nil {
		return nil, err
	}

	newOutputs, err := c.newOutputs(
		changeDistribution,
		funding.ChangeCount,
		derivationPrefix,
		params.Outputs,
		commOut,
		params.RandomizeOutputs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new outputs: %w", err)
	}

	totalAllocated, err := funding.TotalAllocated()
	if err != nil {
		return nil, err
	}

	beef := transaction.NewBeefV2()

	inputBeef, err := beef.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize beef: %w", err)
	}

	err = c.txRepo.CreateTransaction(ctx, &entity.NewTx{
		UserID:      userID,
		Version:     params.Version,
		LockTime:    params.LockTime,
		Status:      wdk.TxStatusUnsigned,
		Reference:   reference,
		IsOutgoing:  true,
		Description: params.Description,
		Satoshis:    satoshi.MustSubtract(funding.ChangeAmount, totalAllocated).Int64(),
		Outputs:     newOutputs,
		ReservedOutputIDs: slices.Map(funding.AllocatedUTXOs, func(utxo *UTXO) uint {
			return utxo.OutputID
		}),
		Labels:    params.Labels,
		InputBeef: inputBeef,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	resultInputs, err := c.resultInputs(ctx, funding.AllocatedUTXOs, params.IncludeInputSourceRawTxs)
	if err != nil {
		return nil, err
	}

	return &wdk.StorageCreateActionResult{
		Reference:        reference,
		Version:          params.Version,
		LockTime:         params.LockTime,
		DerivationPrefix: derivationPrefix,
		Outputs:          c.resultOutputs(newOutputs),
		Inputs:           resultInputs,
		InputBeef:        inputBeef,
	}, nil
}

type serviceChargeOutput struct {
	wdk.ValidCreateActionOutput
	KeyOffset string
}

func (c *create) createCommissionOutput() (*serviceChargeOutput, error) {
	lockingScript, keyOffset, err := c.commission.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate commission script: %w", err)
	}

	return &serviceChargeOutput{
		ValidCreateActionOutput: wdk.ValidCreateActionOutput{
			LockingScript:     primitives.HexString(lockingScript),
			Satoshis:          primitives.SatoshiValue(c.commissionCfg.Satoshis),
			OutputDescription: "Storage Service Charge",
		},
		KeyOffset: keyOffset,
	}, nil
}

func (c *create) targetSat(_ iter.Seq[*wdk.ValidCreateActionInput], xoutputs iter.Seq[*wdk.ValidCreateActionOutput]) (satoshi.Value, error) {
	providedInputs := satoshi.Zero()
	// TODO: sum provided inputs satoshis - but first the values should be found

	providedOutputs, err := satoshi.Sum(seq.Map(xoutputs, func(output *wdk.ValidCreateActionOutput) primitives.SatoshiValue {
		return output.Satoshis
	}))
	if err != nil {
		return 0, fmt.Errorf("failed to sum provided outputs' satoshis: %w", err)
	}

	sub, err := satoshi.Subtract(providedOutputs, providedInputs)
	if err != nil {
		return 0, fmt.Errorf("failed to subtract commission from provided outputs: %w", err)
	}

	return sub, nil
}

func (c *create) txSize(xinputs iter.Seq[*wdk.ValidCreateActionInput], xoutputs iter.Seq[*wdk.ValidCreateActionOutput]) (uint64, error) {
	inputSizes := seqerr.MapSeq(xinputs, func(o *wdk.ValidCreateActionInput) (uint64, error) {
		return o.ScriptLength()
	})

	outputSizes := seqerr.MapSeq(xoutputs, func(o *wdk.ValidCreateActionOutput) (uint64, error) {
		return o.ScriptLength()
	})

	txSize, err := txutils.TransactionSize(inputSizes, outputSizes)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate transaction size: %w", err)
	}

	return txSize, nil
}

func (c *create) randomValues() (derivationPrefix string, reference string, err error) {
	derivationPrefix, err = c.randomDerivation()
	if err != nil {
		return
	}

	reference, err = c.random.Base64(referenceLength)
	if err != nil {
		err = fmt.Errorf("failed to generate random reference: %w", err)
		return
	}

	return
}

func (c *create) newOutputs(
	changeDistribution iter.Seq[satoshi.Value],
	changeCount uint64,
	derivationPrefix string,
	providedOutputs []wdk.ValidCreateActionOutput,
	commissionOutput *serviceChargeOutput,
	randomizeOutputs bool,
) ([]*entity.NewOutput, error) {
	length := must.ConvertToIntFromUnsigned(changeCount) + len(providedOutputs)
	if commissionOutput != nil {
		length++
	}
	len32 := must.ConvertToUInt32(length)

	all := make([]*entity.NewOutput, 0, len32)

	for _, output := range providedOutputs {
		all = append(all, &entity.NewOutput{
			Satoshis:           satoshi.MustFrom(output.Satoshis),
			Basket:             (*string)(output.Basket),
			Spendable:          false, // TODO: Make sure, these outputs turn to spendable during "processAction"
			Change:             false,
			ProvidedBy:         wdk.ProvidedByYou,
			Type:               wdk.OutputTypeCustom,
			LockingScript:      &output.LockingScript,
			CustomInstructions: output.CustomInstructions,
			Description:        string(output.OutputDescription),
		})
	}

	if commissionOutput != nil {
		all = append(all, &entity.NewOutput{
			LockingScript: to.Ptr(commissionOutput.LockingScript),
			Satoshis:      satoshi.MustFrom(commissionOutput.Satoshis),
			Basket:        nil,
			Spendable:     false,
			Change:        false,
			ProvidedBy:    wdk.ProvidedByStorage,
			Type:          wdk.OutputTypeCustom,
			Purpose:       wdk.StorageCommissionPurpose,
		})
	}

	for satoshis := range changeDistribution {
		derivationSuffix, err := c.randomDerivation()
		if err != nil {
			return nil, fmt.Errorf("failed to generate random derivation suffix: %w", err)
		}

		all = append(all, &entity.NewOutput{
			Satoshis:         satoshis,
			Basket:           to.Ptr(wdk.BasketNameForChange),
			Spendable:        false, // TODO: Make sure, these outputs turn to spendable during "processAction"
			Change:           true,
			ProvidedBy:       wdk.ProvidedByStorage,
			Type:             wdk.OutputTypeP2PKH,
			DerivationPrefix: to.Ptr(derivationPrefix),
			DerivationSuffix: to.Ptr(derivationSuffix),
			Purpose:          wdk.ChangePurpose,
		})
	}

	if randomizeOutputs {
		c.random.Shuffle(len(all), func(i, j int) {
			all[i], all[j] = all[j], all[i]
		})
	}

	for vout := uint32(0); vout < len32; vout++ {
		all[vout].Vout = vout
	}

	return all, nil
}

func (c *create) resultOutputs(newOutputs []*entity.NewOutput) []wdk.StorageCreateTransactionSdkOutput {
	resultOutputs := make([]wdk.StorageCreateTransactionSdkOutput, len(newOutputs))
	for i, output := range newOutputs {

		resultOutputs[i] = wdk.StorageCreateTransactionSdkOutput{
			Vout:             output.Vout,
			ProvidedBy:       output.ProvidedBy,
			Purpose:          output.Purpose,
			DerivationSuffix: output.DerivationSuffix,
			ValidCreateActionOutput: wdk.ValidCreateActionOutput{
				Satoshis:           primitives.SatoshiValue(must.ConvertToUInt64(output.Satoshis)),
				OutputDescription:  primitives.String5to2000Bytes(output.Description),
				CustomInstructions: output.CustomInstructions,
				LockingScript:      optional.OfPtr(output.LockingScript).OrZeroValue(),
			},
		}

		if output.Basket != nil {
			resultOutputs[i].Basket = to.Ptr(primitives.StringUnder300(*output.Basket))
		}
	}

	return resultOutputs
}

func (c *create) resultInputs(ctx context.Context, allocatedUTXOs []*UTXO, includeRawTxs bool) ([]wdk.StorageCreateTransactionSdkInput, error) {
	utxos, err := c.outputRepo.FindOutputs(ctx, seq.Map(seq.FromSlice(allocatedUTXOs), func(utxo *UTXO) uint {
		return utxo.OutputID
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to find allocated outputs: %w", err)
	}
	if len(utxos) != len(allocatedUTXOs) {
		return nil, fmt.Errorf("expected %d outputs, got %d", len(allocatedUTXOs), len(utxos))
	}

	resultInputs := make([]wdk.StorageCreateTransactionSdkInput, len(allocatedUTXOs))
	for i, utxo := range utxos {
		if utxo.TxID == nil {
			return nil, fmt.Errorf("missing txid for output %d", i)
		}
		if utxo.LockingScript == nil {
			return nil, fmt.Errorf("missing locking script for output %d", i)
		}
		txID := *utxo.TxID
		resultInputs[i] = wdk.StorageCreateTransactionSdkInput{
			Vin:                   i,
			SourceTxID:            txID,
			SourceVout:            utxo.Vout,
			SourceSatoshis:        utxo.Satoshis,
			SourceLockingScript:   *utxo.LockingScript,
			UnlockingScriptLength: txutils.P2PKHUnlockingScriptLength,
			ProvidedBy:            wdk.ProvidedByStorage,
			Type:                  utxo.Type,
			DerivationPrefix:      utxo.DerivationPrefix,
			DerivationSuffix:      utxo.DerivationSuffix,
			SenderIdentityKey:     utxo.SenderIdentityKey,
		}

		if includeRawTxs {
			sourceTx, err := c.provenTxRepo.FindProvenTxRawTX(ctx, txID)
			if err != nil {
				return nil, fmt.Errorf("failed to find source transaction of TxID = %s: %w", txID, err)
			}
			if len(sourceTx) == 0 {
				return nil, fmt.Errorf("source transaction of TxID = %s is empty", txID)
			}
			resultInputs[i].SourceTransaction = sourceTx
		}

	}
	return resultInputs, nil
}

func (c *create) randomDerivation() (string, error) {
	suffix, err := c.random.Base64(derivationLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate random derivation: %w", err)
	}

	return suffix, nil
}
