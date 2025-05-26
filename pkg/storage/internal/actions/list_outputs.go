package actions

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
)

type listOutputs struct {
	logger       *slog.Logger
	outputsRepo  OutputRepo
	provenTxRepo ProvenTxRepo
}

func newListOutputs(logger *slog.Logger, outputsRepo OutputRepo, provenTxRepo ProvenTxRepo) *listOutputs {
	return &listOutputs{
		logger:       logging.Child(logger, "list_outputs"),
		provenTxRepo: provenTxRepo,
		outputsRepo:  outputsRepo,
	}
}

func (l *listOutputs) ListOutputs(ctx context.Context, auth wdk.AuthID, args *wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
	// TODO: Handle args.Tags
	// TODO: Handle args.TagQueryMode

	// TODO: Handle args.KnownTxids
	// TODO: Handle args.IncludeLockingScripts
	// TODO: Handle args.IncludeCustomInstructions
	// TODO: Handle args.IncludeTags
	// TODO: Handle args.IncludeLabels

	filter := l.toFilterParams(*auth.UserID, args)

	outputModels, totalCount, err := l.outputsRepo.ListAndCountOutputs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error during listing outputs: %w", err)
	}

	outputs := make([]*wdk.WalletOutput, len(outputModels))
	for i, m := range outputModels {
		outputs[i] = l.outputModelToResult(m)
	}

	result := &wdk.ListOutputsResult{
		TotalOutputs: primitives.PositiveInteger(must.ConvertToUInt64(totalCount)),
		Outputs:      outputs,
	}

	if args.IncludeTransactions {
		uniqueTxIDs := l.uniqueTxTDsForAllOutputs(outputModels)

		rawBeef, err := l.provenTxRepo.GetBEEFForTxIDs(ctx, uniqueTxIDs, args.KnownTxids)
		if err != nil {
			return nil, fmt.Errorf("error fetching BEEF data: %w", err)
		}
		beef := primitives.BEEF(rawBeef)
		result.BEEF = &beef
	}

	return result, nil
}

func (l *listOutputs) uniqueTxTDsForAllOutputs(outputModels []*wdk.TableOutput) iter.Seq[string] {
	transactionsWithTxIDs := seq.Filter(seq.FromSlice(outputModels), func(m *wdk.TableOutput) bool {
		return m.TxID != nil && *m.TxID != ""
	})
	allTxIDs := seq.Map(transactionsWithTxIDs, func(m *wdk.TableOutput) string {
		return *m.TxID
	})
	return seq.Uniq(allTxIDs)
}

func (l *listOutputs) toFilterParams(userID int, args *wdk.ListOutputsArgs) entity.ListOutputsFilter {
	return entity.ListOutputsFilter{
		UserID:      userID,
		Basket:      string(args.Basket),
		Limit:       must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Limit, 10000)),
		Offset:      must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Offset, math.MaxInt)),
		IncludeTXID: args.IncludeTransactions,
	}
}

func (l *listOutputs) outputModelToResult(m *wdk.TableOutput) *wdk.WalletOutput {
	result := &wdk.WalletOutput{
		Satoshis:           primitives.SatoshiValue(must.ConvertToUInt64(m.Satoshis)),
		Spendable:          m.Spendable,
		CustomInstructions: m.CustomInstructions,
	}

	if m.TxID != nil {
		outpoint := fmt.Sprintf("%s.%d", *m.TxID, m.Vout)
		result.Outpoint = primitives.OutpointString(outpoint)
	}

	if m.LockingScript != nil {
		result.LockingScript = to.Ptr(primitives.HexString(*m.LockingScript))
	}

	return result
}
