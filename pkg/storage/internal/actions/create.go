package actions

import (
	"context"
	"fmt"
	"github.com/go-softwarelab/common/pkg/slices"
	"iter"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/commission"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	derivationLength = 16
	referenceLength  = 12
)

type CreateActionParams struct {
	Version                  uint32
	LockTime                 uint32
	Description              string
	Labels                   []primitives.StringUnder300
	Outputs                  []wdk.ValidCreateActionOutput
	Inputs                   []wdk.ValidCreateActionInput
	InputBEEF                []byte
	RandomizeOutputs         bool
	IncludeInputSourceRawTxs bool
	TrustSelf                bool
}

func FromValidCreateActionArgs(args *wdk.ValidCreateActionArgs) CreateActionParams {
	return CreateActionParams{
		Version:                  args.Version,
		LockTime:                 args.LockTime,
		Description:              string(args.Description),
		Labels:                   args.Labels,
		Outputs:                  args.Outputs,
		Inputs:                   args.Inputs,
		InputBEEF:                args.InputBEEF,
		RandomizeOutputs:         args.Options.RandomizeOutputs,
		IncludeInputSourceRawTxs: args.IsSignAction && args.IncludeAllSourceTransactions,
		TrustSelf:                args.Options.TrustSelf != nil && *args.Options.TrustSelf == sdk.TrustSelfKnown,
	}
}

type create struct {
	logger        *slog.Logger
	funder        funder.Funder
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
	funder funder.Funder,
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

	processedInputs, err := newInputsProcessor(ctx, c, userID, params.Inputs, params.InputBEEF, params.TrustSelf).
		processInputs()
	if err != nil {
		return nil, fmt.Errorf("failed to process inputs: %w", err)
	}
	xinputs := processedInputs.Inputs
	xoutputs := seq.PointersFromSlice(params.Outputs)

	var commOut *serviceChargeOutput
	if c.commission != nil {
		commOut, err = c.createCommissionOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to collect outputs: %w", err)
		}
		xoutputs = seq.Append(xoutputs, &commOut.ValidCreateActionOutput)
	}

	initialTxSize, err := c.txSize(xinputs.iter(), xoutputs)
	if err != nil {
		return nil, err
	}

	targetSat, err := c.targetSat(xinputs.iter(), xoutputs) // NOTE: Target satoshis can be negative
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target satoshis: %w", err)
	}

	funding, err := c.funder.Fund(ctx, targetSat, initialTxSize, basket, userID, processedInputs.ChangeOutputIDs)
	if err != nil {
		return nil, fmt.Errorf("funding failed: %w", err)
	}

	changeDistribution := txutils.NewChangeDistribution(satoshi.MustFrom(basket.MinimumDesiredUTXOValue), c.random.Uint64).
		Distribute(funding.ChangeOutputsCount, funding.ChangeAmount)

	derivationPrefix, reference, err := c.randomValues()
	if err != nil {
		return nil, err
	}

	newOutputs, err := c.newOutputs(
		changeDistribution,
		funding.ChangeOutputsCount,
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
		return nil, fmt.Errorf("failed to get total allocated inputs: %w", err)
	}

	inputBeef, err := processedInputs.Beef.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize beef: %w", err)
	}

	err = c.txRepo.CreateTransaction(ctx, &entity.NewTx{
		UserID:            userID,
		Version:           params.Version,
		LockTime:          params.LockTime,
		Status:            wdk.TxStatusUnsigned,
		Reference:         reference,
		IsOutgoing:        true,
		Description:       params.Description,
		Satoshis:          satoshi.MustSubtract(funding.ChangeAmount, totalAllocated).Int64(),
		Outputs:           newOutputs,
		ReservedOutputIDs: c.allReservedOutputIDs(funding.AllocatedUTXOs, processedInputs.ChangeOutputIDs),
		Labels:            params.Labels,
		InputBeef:         inputBeef,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	resultInputs, err := c.resultInputs(ctx, funding.AllocatedUTXOs, params.IncludeInputSourceRawTxs, processedInputs.Inputs)
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

func (c *create) targetSat(xinputs iter.Seq[*xinputDefinition], xoutputs iter.Seq[*wdk.ValidCreateActionOutput]) (satoshi.Value, error) {
	providedInputs, err := satoshi.Sum(seq.Map(xinputs, func(input *xinputDefinition) satoshi.Value {
		return input.Satoshis
	}))
	if err != nil {
		return 0, fmt.Errorf("failed to sum provided inputs' satoshis: %w", err)
	}

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

func (c *create) txSize(xinputs iter.Seq[*xinputDefinition], xoutputs iter.Seq[*wdk.ValidCreateActionOutput]) (uint64, error) {
	inputSizes := seqerr.MapSeq(xinputs, func(o *xinputDefinition) (uint64, error) {
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
		tags := slices.Map(output.Tags, func(tag primitives.StringUnder300) string {
			return string(tag)
		})

		all = append(all, &entity.NewOutput{
			Satoshis:           satoshi.MustFrom(output.Satoshis),
			BasketName:         (*string)(output.Basket),
			Spendable:          false,
			Change:             false,
			ProvidedBy:         wdk.ProvidedByYou,
			Type:               wdk.OutputTypeCustom,
			LockingScript:      &output.LockingScript,
			CustomInstructions: output.CustomInstructions,
			Description:        string(output.OutputDescription),
			Tags:               tags,
		})
	}

	if commissionOutput != nil {
		all = append(all, &entity.NewOutput{
			LockingScript: to.Ptr(commissionOutput.LockingScript),
			Satoshis:      satoshi.MustFrom(commissionOutput.Satoshis),
			BasketName:    nil,
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
			BasketName:       to.Ptr(wdk.BasketNameForChange),
			Spendable:        false,
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

func (c *create) resultOutputs(newOutputs []*entity.NewOutput) []*wdk.StorageCreateTransactionSdkOutput {
	resultOutputs := make([]*wdk.StorageCreateTransactionSdkOutput, len(newOutputs))
	for i, output := range newOutputs {

		resultOutputs[i] = &wdk.StorageCreateTransactionSdkOutput{
			Vout:             output.Vout,
			ProvidedBy:       output.ProvidedBy,
			Purpose:          output.Purpose,
			DerivationSuffix: output.DerivationSuffix,
			ValidCreateActionOutput: wdk.ValidCreateActionOutput{
				Satoshis:           primitives.SatoshiValue(must.ConvertToUInt64(output.Satoshis)),
				OutputDescription:  primitives.String5to2000Bytes(output.Description),
				CustomInstructions: output.CustomInstructions,
				LockingScript:      optional.OfPtr(output.LockingScript).OrZeroValue(),
				Basket:             (*primitives.StringUnder300)(output.BasketName),
			},
		}
	}

	return resultOutputs
}

func (c *create) resultInputs(ctx context.Context, allocatedUTXOs []*funder.UTXO, includeRawTxs bool, xinputs xinputDefinitions) ([]*wdk.StorageCreateTransactionSdkInput, error) {
	utxos, err := c.outputRepo.FindOutputs(ctx, seq.Map(seq.FromSlice(allocatedUTXOs), func(utxo *funder.UTXO) uint {
		return utxo.OutputID
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to find allocated outputs: %w", err)
	}
	if len(utxos) != len(allocatedUTXOs) {
		return nil, fmt.Errorf("expected %d outputs, got %d", len(allocatedUTXOs), len(utxos))
	}

	resultInputs := make([]*wdk.StorageCreateTransactionSdkInput, 0, len(allocatedUTXOs)+len(xinputs))

	var vin int
	for unknownProvided := range xinputs.providedByUserAndUnknown() {
		input := &wdk.StorageCreateTransactionSdkInput{
			Vin:                   vin,
			SourceTxID:            unknownProvided.Outpoint.TxID,
			SourceVout:            unknownProvided.Outpoint.Vout,
			SourceSatoshis:        unknownProvided.Satoshis.Int64(),
			SourceLockingScript:   unknownProvided.LockingScript,
			UnlockingScriptLength: unknownProvided.UnlockingScriptLength,
			ProvidedBy:            wdk.ProvidedByYou,
			Type:                  wdk.OutputTypeCustom,
		}

		resultInputs = append(resultInputs, input)
		vin++
	}

	for knownProvided := range xinputs.knownOutputs() {
		input, err := c.resultInputForKnownUTXO(ctx, vin, knownProvided, includeRawTxs, wdk.ProvidedByYouAndStorage)
		if err != nil {
			return nil, fmt.Errorf("failed to create result input for provided-by-user and known UTXO: %w", err)
		}

		resultInputs = append(resultInputs, input)
		vin++
	}

	for _, allocatedOutputs := range utxos {
		input, err := c.resultInputForKnownUTXO(ctx, vin, allocatedOutputs, includeRawTxs, wdk.ProvidedByStorage)
		if err != nil {
			return nil, fmt.Errorf("failed to create result input for known UTXO: %w", err)
		}

		resultInputs = append(resultInputs, input)
		vin++
	}

	return resultInputs, nil
}

func (c *create) resultInputForKnownUTXO(ctx context.Context, vin int, utxo *entity.Output, includeRawTxs bool, providedBy wdk.ProvidedBy) (*wdk.StorageCreateTransactionSdkInput, error) {
	if utxo.TxID == nil {
		return nil, fmt.Errorf("missing txid for outputID %d", utxo.ID)
	}

	if utxo.LockingScript == nil {
		return nil, fmt.Errorf("missing locking script for outputID %d and TxID %s", utxo.ID, *utxo.TxID)
	}

	txID := *utxo.TxID
	result := wdk.StorageCreateTransactionSdkInput{
		Vin:                   vin,
		SourceTxID:            txID,
		SourceVout:            utxo.Vout,
		SourceSatoshis:        utxo.Satoshis,
		SourceLockingScript:   *utxo.LockingScript,
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(txutils.P2PKHUnlockingScriptLength)),
		ProvidedBy:            providedBy,
		Type:                  wdk.OutputType(utxo.Type),
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
		result.SourceTransaction = sourceTx
	}
	return &result, nil
}

func (c *create) randomDerivation() (string, error) {
	suffix, err := c.random.Base64(derivationLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate random derivation: %w", err)
	}

	return suffix, nil
}

func (c *create) allReservedOutputIDs(allocated []*funder.UTXO, providedOutputsIDs []uint) []uint {
	ids := make([]uint, 0, len(allocated)+len(providedOutputsIDs))
	ids = append(ids, providedOutputsIDs...)
	for _, utxo := range allocated {
		ids = append(ids, utxo.OutputID)
	}
	return ids
}
