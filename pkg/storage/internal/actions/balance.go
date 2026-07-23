package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-softwarelab/common/pkg/must"
	"go.opentelemetry.io/otel/attribute"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// sumSpendableBasketSatoshis returns the sum of satoshis for spendable outputs in the
// given basket for the user. Empty basket defaults to BasketNameForChange ("default").
// Shared by GetBalance and the ListOutputs wallet-balance spec-op path.
func sumSpendableBasketSatoshis(ctx context.Context, outputsRepo OutputRepo, userID int, basket string) (uint64, error) {
	if basket == "" {
		basket = wdk.BasketNameForChange
	}

	filter := entity.ListOutputsFilter{
		UserID: userID,
		Basket: basket,
		// Limit -1 means no limit (fetch all spendable outputs for the basket).
		Limit: -1,
	}

	var outputModels []*pkgentity.Output
	var err error
	outputModels, _, err = outputsRepo.ListAndCountOutputs(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("error listing outputs for balance: %w", err)
	}

	var totalSatoshis uint64
	for _, o := range outputModels {
		totalSatoshis += must.ConvertToUInt64(o.Satoshis)
	}
	return totalSatoshis, nil
}

// GetBalance returns total spendable satoshis in the given basket for the user.
func (l *listOutputs) GetBalance(ctx context.Context, userID int, basket string) (balance uint64, err error) {
	ctx, span := tracing.StartTracing(
		ctx, "StorageActions-GetBalance",
		attribute.Int("userID", userID),
		attribute.String("basket", basket),
	)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	l.logger.DebugContext(
		ctx, "GetBalance",
		slog.Int("userID", userID),
		slog.String("basket", basket),
	)

	balance, err = sumSpendableBasketSatoshis(ctx, l.outputsRepo, userID, basket)
	if err != nil {
		return 0, err
	}
	return balance, nil
}
