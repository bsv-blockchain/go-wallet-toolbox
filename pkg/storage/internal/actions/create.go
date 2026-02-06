package actions

import (
	"context"
	"encoding/hex"
	"fmt"
	"iter"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/commission"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
)

const (
	derivationLength = 16
	referenceLength  = 12
)

type CreateActionParams struct {
	Version                  uint32
	LockTime                 uint32
	Description              string
	KnownTxIDs               []primitives.TXIDHexString
	Labels                   []primitives.StringUnder300
	Outputs                  []wdk.ValidCreateActionOutput
	Inputs                   []wdk.ValidCreateActionInput
	NoSendChange             []wdk.OutPoint
	InputBEEF                []byte
	RandomizeOutputs         bool
	IncludeInputSourceRawTxs bool
	TrustSelf                bool
	IsNoSend                 bool
	IsDelayed                bool
	Reference                string
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
		IsNoSend:                 args.IsNoSend,
		NoSendChange:             args.Options.NoSendChange,
		IsDelayed:                args.IsDelayed,
		KnownTxIDs:               args.Options.KnownTxids,
		Reference:                args.Reference,
	}
}

type create struct {
	logger         *slog.Logger
	funder         funder.Funder
	basketRepo     BasketRepo
	txRepo         TransactionsRepo
	outputRepo     OutputRepo
	knownTxRepo    KnownTxRepo
	commissionRepo CommissionRepo
	commission     *commission.ScriptGenerator
	commissionCfg  defs.Commission
	random         wdk.Randomizer
	chaintracker   chaintracker.ChainTracker
	beefVerifier   wdk.BeefVerifier
}

func newCreateAction(
	logger *slog.Logger,
	funder funder.Funder,
	commissionCfg defs.Commission,
	basketRepo BasketRepo,
	txRepo TransactionsRepo,
	outputRepo OutputRepo,
	knownTxRepo KnownTxRepo,
	commissionRepo CommissionRepo,
	random wdk.Randomizer,
	chaintracker chaintracker.ChainTracker,
	beefVerifier wdk.BeefVerifier,
) *create {
	logger = logging.Child(logger, "createAction")
	c := &create{
		logger:         logger,
		funder:         funder,
		basketRepo:     basketRepo,
		txRepo:         txRepo,
		commissionCfg:  commissionCfg,
		outputRepo:     outputRepo,
		knownTxRepo:    knownTxRepo,
		commissionRepo: commissionRepo,
		random:         random,
		chaintracker:   chaintracker,
		beefVerifier:   beefVerifier,
	}

	if commissionCfg.Enabled() {
		c.commission = commission.NewScriptGenerator(string(commissionCfg.PubKeyHex))
	}

	return c
}

func (c *create) Create(ctx context.Context, userID int, params CreateActionParams) (*wdk.StorageCreateActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-Create", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var reference string
	if params.Reference != "" {
		reference = params.Reference
	} else {
		reference, err = c.randomReference()
		if err != nil {
			return nil, fmt.Errorf("failed to generate reference number: %w", err)
		}
	}

	c.logger.DebugContext(ctx, "Searching for change basket",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("basketName", wdk.BasketNameForChange),
	)

	basket, err := c.basketRepo.FindBasketByName(ctx, userID, wdk.BasketNameForChange)
	if err != nil {
		return nil, fmt.Errorf("failed to find basket for change: %w", err)
	}
	if basket == nil {
		return nil, fmt.Errorf("basket for change (%s) not found", wdk.BasketNameForChange)
	}

	priorityOutputs, err := c.getNoSendOutputs(ctx, userID, params.IsNoSend, params.NoSendChange, reference)
	if err != nil {
		return nil, fmt.Errorf("failed to create priority outputs: %w", err)
	}

	c.logger.DebugContext(ctx, "Processing inputs",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Int("providedInputCount", len(params.Inputs)),
		slog.Bool("trustSelf", params.TrustSelf),
		slog.Int("inputBEEFSize", len(params.InputBEEF)),
	)

	inputProcessor, err := newInputsProcessor(ctx, c, userID, reference, params.Inputs, params.InputBEEF, params.TrustSelf, c.beefVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to create inputs processor: %w", err)
	}

	processedInputs, err := inputProcessor.processInputs()
	if err != nil {
		return nil, fmt.Errorf("failed to process inputs: %w", err)
	}

	xinputs := processedInputs.Inputs
	xoutputs := seq.PointersFromSlice(params.Outputs)

	var commOut *serviceChargeOutput
	if c.commission != nil {
		c.logger.DebugContext(ctx, "Creating commission output",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.Uint64("commissionSatoshis", c.commissionCfg.Satoshis),
		)

		commOut, err = c.createCommissionOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to collect outputs: %w", err)
		}

		c.logger.DebugContext(ctx, "Commission output created",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.Uint64("commissionSatoshis", uint64(commOut.Satoshis)),
			slog.String("commissionBasket", string(to.Value(commOut.Basket))),
			slog.String("commissionKeyOffset", commOut.KeyOffset),
		)
		xoutputs = seq.Append(xoutputs, &commOut.ValidCreateActionOutput)
	} else {
		c.logger.DebugContext(ctx, "Commission disabled, skipping commission output creation",
			logging.UserID(userID),
			logging.Reference(reference),
		)
	}

	c.logger.DebugContext(ctx, "Calculating transaction size",
		logging.UserID(userID),
		logging.Reference(reference),
	)
	initialTxSize, err := c.txSize(xinputs.iter(), xoutputs)
	if err != nil {
		return nil, err
	}

	c.logger.DebugContext(ctx, "Calculating target satoshis",
		logging.UserID(userID),
		logging.Reference(reference),
	)
	targetSat, err := c.targetSat(xinputs.iter(), xoutputs) // NOTE: Target satoshis can be negative
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target satoshis: %w", err)
	}

	c.logger.DebugContext(ctx, "Transaction size and target calculated",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Uint64("initialTxSize", initialTxSize),
		logging.Number("targetSatoshis", targetSat),
	)

	c.logger.InfoContext(ctx, "Funding transaction",
		logging.UserID(userID),
		logging.Reference(reference),
		logging.Number("targetSatoshis", targetSat),
		slog.Uint64("initialTxSize", initialTxSize),
		slog.Uint64("basketMinimumUTXOValue", basket.MinimumDesiredUTXOValue),
	)

	includeUTXOsInSendingState := params.IsDelayed

	outputCount := uint64(len(params.Outputs))
	if commOut != nil {
		outputCount++
	}

	funding, err := c.funder.Fund(ctx, targetSat, initialTxSize, outputCount, basket, userID, processedInputs.ChangeOutputIDs, priorityOutputs, includeUTXOsInSendingState)
	if err != nil {
		return nil, fmt.Errorf("funding failed: %w", err)
	}

	c.logger.InfoContext(ctx, "Transaction funding completed",
		logging.UserID(userID),
		logging.Reference(reference),
		logging.Number("changeAmount", funding.ChangeAmount),
		slog.Uint64("changeOutputsCount", funding.ChangeOutputsCount),
		slog.Int("allocatedUTXOsCount", len(funding.AllocatedUTXOs)),
		logging.Number("fee", funding.Fee),
	)

	c.logger.DebugContext(ctx, "Creating change distribution",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Uint64("minimumDesiredUTXOValue", basket.MinimumDesiredUTXOValue),
		slog.Uint64("changeOutputsCount", funding.ChangeOutputsCount),
		logging.Number("changeAmount", funding.ChangeAmount),
	)

	changeDistribution := txutils.NewChangeDistribution(satoshi.MustFrom(basket.MinimumDesiredUTXOValue), c.random.Uint64).
		Distribute(funding.ChangeOutputsCount, funding.ChangeAmount)

	derivationPrefix, err := c.randomDerivation()
	if err != nil {
		return nil, err
	}

	c.logger.DebugContext(ctx, "Generated derivation prefix",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Int("derivationPrefixLength", len(derivationPrefix)),
	)

	c.logger.DebugContext(ctx, "Creating new outputs",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Int("providedOutputsCount", len(params.Outputs)),
		slog.Bool("randomizeOutputs", params.RandomizeOutputs),
		slog.Bool("hasCommissionOutput", commOut != nil),
	)

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

	c.logger.DebugContext(ctx, "Created new outputs",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Int("totalOutputsCount", len(newOutputs)),
	)

	totalAllocated, err := funding.TotalAllocated()
	if err != nil {
		return nil, fmt.Errorf("failed to get total allocated inputs: %w", err)
	}

	inputBeef, err := processedInputs.Beef.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize beef: %w", err)
	}

	c.logger.DebugContext(ctx, "Saving transaction in database",
		logging.UserID(userID),
		logging.Reference(reference),
		logging.Number("txVersion", params.Version),
		logging.Number("txLockTime", params.LockTime),
		logging.Number("totalAllocated", totalAllocated),
		logging.Number("changeAmount", funding.ChangeAmount),
		slog.String("satoshis", fmt.Sprintf("%v - %v", funding.ChangeAmount, totalAllocated)),
		slog.String("description", params.Description),
		slog.Int("inputBeefSize", len(inputBeef)),
	)

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
		Commission:        c.createCommissionEntity(userID, commOut),
		UTXOStatus:        wdk.UTXOStatusUnknown,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	c.logger.InfoContext(ctx, "Transaction saved in database successfully",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("status", string(wdk.TxStatusUnsigned)),
	)

	c.logger.DebugContext(ctx, "Creating result inputs",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Bool("includeInputSourceRawTxs", params.IncludeInputSourceRawTxs),
	)

	resultInputs, err := c.resultInputs(ctx, funding.AllocatedUTXOs, params.IncludeInputSourceRawTxs, processedInputs.Inputs)
	if err != nil {
		return nil, err
	}

	c.logger.DebugContext(ctx, "CreateAction process completed",
		logging.UserID(userID),
		logging.Reference(reference),
		logging.Number("txVersion", params.Version),
		logging.Number("txLockTime", params.LockTime),
		slog.Int("outputsCount", len(newOutputs)),
		slog.Int("inputBeefSize", len(inputBeef)),
		slog.Int("inputsCount", len(resultInputs)),
	)

	beef, err := c.mergeAllocatedUTXOs(ctx, processedInputs.Beef, funding.AllocatedUTXOs, params.KnownTxIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create BEEF with allocated UTXOs: %w", err)
	}

	return &wdk.StorageCreateActionResult{
		Reference:               reference,
		Version:                 params.Version,
		LockTime:                params.LockTime,
		DerivationPrefix:        derivationPrefix,
		Outputs:                 c.resultOutputs(newOutputs),
		Inputs:                  resultInputs,
		InputBeef:               beef,
		NoSendChangeOutputVouts: c.changeOutputVoutsResult(params.IsNoSend, newOutputs...),
	}, nil
}

func (c *create) getNoSendOutputs(ctx context.Context, userID int, isNoSend bool, noSendChange []wdk.OutPoint, ref string) ([]*pkgentity.Output, error) {
	logger := c.logger.With(
		logging.Reference(ref),
		slog.Bool("isNoSendParam", isNoSend),
		slog.Int("noSendChangeParam", len(noSendChange)),
		logging.UserID(userID),
	)

	if isNoSend && len(noSendChange) == 0 {
		logger.DebugContext(ctx, "NoSendOutputs not provided")
		return []*pkgentity.Output{}, nil
	}

	outputs, err := c.outputRepo.FindOutputsByOutpoints(ctx, userID, noSendChange)
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs by outpoints: %w", err)
	}

	logger.DebugContext(ctx, "Entity outputs successfully returned from the repository")

	if len(noSendChange) != len(outputs) {
		return nil, fmt.Errorf("failed to validate outputs: the number of outputs (%d) doesn't match the number of outpoints (%d)", len(outputs), len(noSendChange))
	}

	err = validate.NoSendChangeOutputs(outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to validate no send change outputs: %w", err)
	}

	logger.DebugContext(ctx, "Entity outputs (no send change outputs) successfully validated")

	return outputs, nil
}

func (c *create) changeOutputVoutsResult(isNoSend bool, newOutputs ...*entity.NewOutput) []int {
	if !isNoSend {
		return nil
	}

	var vouts []int
	for _, output := range newOutputs {
		if output.IsChangeOutputVout() {
			vouts = append(vouts, int(output.Vout))
		}
	}
	return vouts
}

type serviceChargeOutput struct {
	wdk.ValidCreateActionOutput
	KeyOffset          string
	LockingScriptBytes []byte
}

func (c *create) createCommissionOutput() (*serviceChargeOutput, error) {
	lockingScript, keyOffset, err := c.commission.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate commission script: %w", err)
	}

	commOut := &serviceChargeOutput{
		ValidCreateActionOutput: wdk.ValidCreateActionOutput{
			LockingScript:     primitives.HexString(lockingScript.String()),
			Satoshis:          primitives.SatoshiValue(c.commissionCfg.Satoshis),
			OutputDescription: "Storage Service Charge",
		},
		KeyOffset:          keyOffset,
		LockingScriptBytes: lockingScript.Bytes(),
	}

	return commOut, nil
}

func (c *create) createCommissionEntity(userID int, commOut *serviceChargeOutput) *pkgentity.Commission {
	if commOut == nil {
		return nil
	}

	return &pkgentity.Commission{
		UserID:        userID,
		Satoshis:      c.commissionCfg.Satoshis,
		KeyOffset:     commOut.KeyOffset,
		IsRedeemed:    false,
		LockingScript: commOut.LockingScriptBytes,
	}
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
			Spendable:          true,
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
			Spendable:        true,
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
				Tags: slices.Map(output.Tags, func(tag string) primitives.StringUnder300 {
					return primitives.StringUnder300(tag)
				}),
			},
		}
	}

	return resultOutputs
}

func (c *create) resultInputs(ctx context.Context, allocatedUTXOs []*funder.UTXO, includeRawTxs bool, xinputs xinputDefinitions) ([]*wdk.StorageCreateTransactionSdkInput, error) {
	utxos, err := c.outputRepo.FindOutputsByIDs(ctx, seq.Map(seq.FromSlice(allocatedUTXOs), func(utxo *funder.UTXO) uint {
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
			SourceLockingScript:   hex.EncodeToString(unknownProvided.LockingScript),
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

func (c *create) resultInputForKnownUTXO(ctx context.Context, vin int, utxo *pkgentity.Output, includeRawTxs bool, providedBy wdk.ProvidedBy) (*wdk.StorageCreateTransactionSdkInput, error) {
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
		SourceLockingScript:   hex.EncodeToString(utxo.LockingScript),
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(txutils.P2PKHUnlockingScriptLength)),
		ProvidedBy:            providedBy,
		Type:                  wdk.OutputType(utxo.Type),
		DerivationPrefix:      utxo.DerivationPrefix,
		DerivationSuffix:      utxo.DerivationSuffix,
		SenderIdentityKey:     utxo.SenderIdentityKey,
	}

	if includeRawTxs {
		sourceTx, err := c.knownTxRepo.FindKnownTxRawTx(ctx, txID)
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

func (c *create) randomReference() (string, error) {
	reference, err := c.random.Base64(referenceLength)
	if err != nil {
		err = fmt.Errorf("failed to generate random reference: %w", err)
		return "", err
	}
	return reference, nil
}

func (c *create) allReservedOutputIDs(allocated []*funder.UTXO, providedOutputsIDs []uint) []uint {
	ids := make([]uint, 0, len(allocated)+len(providedOutputsIDs))
	ids = append(ids, providedOutputsIDs...)
	for _, utxo := range allocated {
		ids = append(ids, utxo.OutputID)
	}
	return ids
}

func (c *create) mergeAllocatedUTXOs(
	ctx context.Context,
	inputBeef *transaction.Beef,
	allocatedUTXOs []*funder.UTXO,
	knownTxIDs primitives.TXIDHexStrings,
) (primitives.ExplicitByteArray, error) {
	txIDs, err := c.outputRepo.FindTxIDsByOutputIDs(ctx, seq.Map(seq.FromSlice(allocatedUTXOs), func(utxo *funder.UTXO) uint {
		return utxo.OutputID
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to find allocated outputs: %w", err)
	}

	beefTx, err := c.knownTxRepo.GetBEEFForTxIDs(ctx, seq.FromSlice(txIDs), entity.WithMergeToBEEF(inputBeef), entity.WithKnownTxIDs(knownTxIDs.ToStringSlice()...))
	if err != nil {
		return nil, fmt.Errorf("failed to get BEEF for allocated UTXOs: %w", err)
	}

	beefBytes, err := beefTx.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to return the BEEF BRC-96 as a byte slice: %w", err)
	}

	return beefBytes, nil
}
