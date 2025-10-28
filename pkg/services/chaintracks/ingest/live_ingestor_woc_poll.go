package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	genesisAsPrevBlockHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

type LiveIngestorWocPoll struct {
	logger *slog.Logger
	config defs.WOCPollIngestorConfig
	resty  *resty.Client
}

func NewLiveIngestorWocPoll(logger *slog.Logger, config defs.WOCPollIngestorConfig, opts ...func(options *ClientOptions)) *LiveIngestorWocPoll {
	logger = logging.Child(logger, "live_ingestor_woc_poll")

	options := to.OptionsWithDefault(ClientOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}, opts...)

	url, err := whatsonchain.MakeBaseURL(config.Chain)
	if err != nil {
		panic(fmt.Sprintf("failed to build base URL for WhatsOnChain: %s", err.Error()))
	}

	restyClient := options.RestyClientFactory.New()
	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		SetBaseURL(url)

	return &LiveIngestorWocPoll{
		logger: logger,
		config: config,
		resty:  restyClient,
	}
}

func (ing *LiveIngestorWocPoll) GetHeaderByHash(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
	path := fmt.Sprintf("/block/%s/header", hash)

	var hdrResp WOCBlockHeaderDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&hdrResp).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		if res.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("block header not found for hash %s: %w", hash, wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("unexpected status code %d fetching block header", res.StatusCode())
	}

	bitsNum, err := ing.bitsStrToUint32(hdrResp.Bits)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %s: %w", hdrResp.Bits, err)
	}

	if hdrResp.PrevBlock == "" {
		hdrResp.PrevBlock = genesisAsPrevBlockHash
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      hdrResp.Version,
			PreviousHash: hdrResp.PrevBlock,
			MerkleRoot:   hdrResp.MerkleRoot,
			Time:         hdrResp.Time,
			Bits:         bitsNum,
			Nonce:        hdrResp.Nonce,
		},
		Hash:   hdrResp.Hash,
		Height: hdrResp.Height,
	}, nil
}

func (ing *LiveIngestorWocPoll) bitsStrToUint32(bitsStr string) (uint32, error) {
	bitsNum, err := strconv.ParseUint(bitsStr, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid bits value %s: %w", bitsStr, err)
	}

	return uint32(bitsNum), nil
}
