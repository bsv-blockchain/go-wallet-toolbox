package actions

import (
	"context"
	"encoding/hex"
	"fmt"
	"iter"
	"log/slog"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

type listOutputs struct {
	logger      *slog.Logger
	outputsRepo OutputRepo
	knownTxRepo KnownTxRepo
}

func newListOutputs(logger *slog.Logger, outputsRepo OutputRepo, knownTxRepo KnownTxRepo) *listOutputs {
	return &listOutputs{
		logger:      logging.Child(logger, "list_outputs"),
		knownTxRepo: knownTxRepo,
		outputsRepo: outputsRepo,
	}
}

func (l *listOutputs) ListOutputs(ctx context.Context, auth wdk.AuthID, args *wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
	// TODO: Handle args.KnownTxids
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

		beef, err := l.knownTxRepo.GetBEEFForTxIDs(
			ctx,
			uniqueTxIDs,
			entity.WithKnownTxIDs(args.KnownTxids...),
			entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...),
		)
		if err != nil {
			return nil, fmt.Errorf("error fetching BEEF data: %w", err)
		}

		rawBeef, err := beef.Bytes()
		if err != nil {
			return nil, fmt.Errorf("error converting BEEF to bytes: %w", err)
		}

		result.BEEF = to.Ptr[primitives.BEEF](rawBeef)
	}

	return result, nil
}

func (l *listOutputs) uniqueTxTDsForAllOutputs(outputModels []*pkgentity.Output) iter.Seq[string] {
	transactionsWithTxIDs := seq.Filter(seq.FromSlice(outputModels), func(m *pkgentity.Output) bool {
		return m.TxID != nil && *m.TxID != ""
	})
	allTxIDs := seq.Map(transactionsWithTxIDs, func(m *pkgentity.Output) string {
		return *m.TxID
	})
	return seq.Uniq(allTxIDs)
}

func (l *listOutputs) toFilterParams(userID int, args *wdk.ListOutputsArgs) entity.ListOutputsFilter {
	return entity.ListOutputsFilter{
		IncludeSpent:              false,
		UserID:                    userID,
		Basket:                    string(args.Basket),
		Limit:                     must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Limit, validate.MaxPaginationLimit)),
		Offset:                    must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Offset, validate.MaxPaginationOffset)),
		IncludeTags:               args.IncludeTags,
		IncludeLockingScripts:     args.IncludeLockingScripts,
		IncludeCustomInstructions: args.IncludeCustomInstructions,
		TagsQueryMode:             args.TagQueryMode.MustGetValue(),

		Tags: slices.Map(args.Tags, func(tag primitives.StringUnder300) string {
			return string(tag)
		}),
	}
}

func (l *listOutputs) outputModelToResult(m *pkgentity.Output) *wdk.WalletOutput {
	result := &wdk.WalletOutput{
		Satoshis:           primitives.SatoshiValue(must.ConvertToUInt64(m.Satoshis)),
		Spendable:          m.Spendable,
		CustomInstructions: m.CustomInstructions,
		Tags: slices.Map(m.Tags, func(tag string) primitives.StringUnder300 {
			return primitives.StringUnder300(tag)
		}),
	}

	if m.TxID != nil {
		result.Outpoint = primitives.NewOutpointString(*m.TxID, m.Vout)
	}

	if m.LockingScript != nil {
		result.LockingScript = to.Ptr(primitives.HexString(hex.EncodeToString(m.LockingScript)))
	}

	return result
}
