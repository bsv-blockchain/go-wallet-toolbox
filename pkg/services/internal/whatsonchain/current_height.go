package whatsonchain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

// GET /v1/bsv/<network>/chain/info
type chainInfoDTO struct {
	Blocks uint64 `json:"blocks"`
}

// CurrentHeight returns the current best-chain height.
func (woc *WhatsOnChain) CurrentHeight(ctx context.Context) (_ uint32, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-CurrentHeight", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var info chainInfoDTO

	url, err := chainInfoURL(woc.url)
	if err != nil {
		return 0, fmt.Errorf("failed to build chain info URL: %w", err)
	}
	res, err := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&info).
		Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch chain info: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("unexpected response from WhatsOnChain (URL: %s): status %d", url, res.StatusCode())
	}
	if info.Blocks == 0 {
		return 0, fmt.Errorf("WhatsOnChain returned height 0")
	}

	height, err := to.UInt32(info.Blocks)
	if err != nil {
		return 0, fmt.Errorf("invalid height %d in WhatsOnChain response: %w", info.Blocks, err)
	}
	return height, nil
}
