package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
)

// Custom ARC defined http status codes
const (
	StatusNotExtendedFormat             = 460
	StatusFeeTooLow                     = 465
	StatusCumulativeFeeValidationFailed = 473
)

type Config = defs.ARC

const ServiceName = defs.ArcServiceName

type Service struct {
	logger           *slog.Logger
	httpClient       *resty.Client
	config           Config
	broadcastURL     string
	queryTxURL       string
	broadcastHeaders httpx.Headers
}

// New creates a new arc service.
func New(logger *slog.Logger, httpClient *resty.Client, config Config) *Service {
	logger = logging.Child(logger, "arc")

	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.Token).
		Set("XDeployment-ID").OrDefault(config.DeploymentID, "go-wallet-toolbox#"+time.Now().Format("20060102150405"))

	httpClient = httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	service := &Service{
		logger:     logger,
		httpClient: httpClient,
		config:     config,

		broadcastURL: config.URL + "/v1/tx",
		broadcastHeaders: httpx.NewHeaders().
			Set("X-CallbackUrl").IfNotEmpty(config.CallbackURL).
			Set("X-CallbackToken").IfNotEmpty(config.CallbackToken).
			Set("X-WaitFor").IfNotEmpty(config.WaitFor),

		queryTxURL: config.URL + "/v1/tx/{txID}",
	}

	return service
}

// PostBEEF attempts to post beef with given txIDs
func (s *Service) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
	err := s.validateBEEF(beef)
	if err != nil {
		return nil, err
	}

	beefHex, err := s.toHex(beef)
	if err != nil {
		return nil, err
	}

	response, err := s.broadcast(ctx, beefHex)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast beef: %w", err)
	}

	var resultsForTxID iter.Seq[wdk.PostedTxID]
	if response != nil {
		resultsForTxID = s.getMissingTxIDResults(ctx, response, txIDs)
	} else {
		resultsForTxID = s.getTxIDResults(ctx, txIDs)
	}

	results := seq.Collect(resultsForTxID)
	for i := range results {
		withBroadcastNote(&results[i], beefHex, txIDs)
	}

	return &wdk.PostedBEEF{
		TxIDResults: results,
	}, nil
}

func (s *Service) getMissingTxIDResults(ctx context.Context, txInfo *TXInfo, txIDs []string) iter.Seq[wdk.PostedTxID] {
	txIDsWithMissingTxInfo := seq.Filter(seq.FromSlice(txIDs), func(txID string) bool {
		return txInfo.TxID != txID
	})

	txsData := internal.MapParallel(ctx, txIDsWithMissingTxInfo, s.getTransactionData)

	subjectTxResult := internal.NewNamedResult(txInfo.TxID, types.SuccessResult(txInfo))
	txsData = seq.Prepend(txsData, subjectTxResult)

	return seq.Map(txsData, toResultForPostTxID)
}

func (s *Service) getTxIDResults(ctx context.Context, txIDs []string) iter.Seq[wdk.PostedTxID] {
	txIDsWithMissingTxInfo := seq.FromSlice(txIDs)

	txsData := internal.MapParallel(ctx, txIDsWithMissingTxInfo, s.getTransactionData)

	return seq.Map(txsData, toResultForPostTxID)
}

func withBroadcastNote(result *wdk.PostedTxID, beefHex string, txIDs []string) {
	switch result.Result {
	case wdk.PostedTxIDResultSuccess, wdk.PostedTxIDResultAlreadyKnown:
		result.Notes = history.NewBuilder().PostBeefSuccess(ServiceName, txIDs).Note().AsList()
	case wdk.PostedTxIDResultError, wdk.PostedTxIDResultDoubleSpend, wdk.PostedTxIDResultMissingInputs:
		fallthrough
	default:
		msg := fmt.Sprintf("broadcasted beef with problematic result %s", result.Result)
		if result.Error != nil {
			msg += fmt.Sprintf(" and error: %v", result.Error)
		}
		result.Notes = history.NewBuilder().PostBeefError(ServiceName, history.Hex(beefHex), txIDs, msg).Note().AsList()
	}
}

func toResultForPostTxID(it *internal.NamedResult[*TXInfo]) wdk.PostedTxID {
	if it.IsError() {
		return wdk.PostedTxID{
			TxID:   it.Name(),
			Result: wdk.PostedTxIDResultError,
			Error:  it.MustGetError(),
		}
	}
	info := it.MustGetValue()

	doubleSpend := info.TXStatus == DoubleSpendAttempted
	result := wdk.PostedTxID{
		Result:       to.IfThen(doubleSpend, wdk.PostedTxIDResultError).ElseThen(wdk.PostedTxIDResultSuccess),
		TxID:         it.Name(),
		DoubleSpend:  doubleSpend,
		BlockHash:    info.BlockHash,
		BlockHeight:  info.BlockHeight,
		CompetingTxs: info.CompetingTxs,
	}

	if is.NotBlankString(info.MerklePath) {
		merklePath, err := transaction.NewMerklePathFromHex(info.MerklePath)
		if err != nil {
			result.Error = err
			result.Result = wdk.PostedTxIDResultError
		} else {
			result.MerklePath = merklePath
		}
	}

	dataBytes, err := json.Marshal(info)
	if err != nil {
		// fallback to string representation in very unlikely case of json marshal error.
		result.Data = fmt.Sprintf("%+v", info)
	} else {
		result.Data = string(dataBytes)
	}

	return result
}

func (s *Service) validateBEEF(beef *transaction.Beef) error {
	if beef == nil {
		return fmt.Errorf("cannot broadcast nil beef")
	}

	if len(beef.Transactions) == 0 {
		return fmt.Errorf("cannot broadcast empty beef")
	}

	beefTxs := seq2.Values(seq2.FromMap(beef.Transactions))
	canBeSerializedToBEEFV1 := seq.Every(beefTxs, func(tx *transaction.BeefTx) bool {
		return tx.DataFormat != transaction.TxIDOnly && tx.Transaction != nil
	})

	if !canBeSerializedToBEEFV1 {
		return fmt.Errorf("arc is not supporting beef v2 and provided beef cannot be converted to v1")
	}
	return nil
}

func (s *Service) getTransactionData(ctx context.Context, txID string) *internal.NamedResult[*TXInfo] {
	txInfo, err := s.queryTransaction(ctx, txID)
	if err != nil {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("arc query tx %s failed: %w", txID, err)))
	}

	if txInfo == nil {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("not found tx %s in arc", txID)))
	}

	if txInfo.TxID != txID {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("got response for tx %s while querying for %s", txInfo.TxID, txID)))
	}

	return internal.NewNamedResult(txID, types.SuccessResult(txInfo))
}

func (s *Service) toHex(beef *transaction.Beef) (string, error) {
	// This is a temporary fix on BEEF until the merge beef will work properly and will bind the tx with bump.
	s.bindBumpsAndTransactions(beef)

	// This is a temporary solution until go-sdk properly implements BEEF serialization
	// It searches for the subject transaction in transaction.Beef and serializes this one to BEEF hex.
	// For now, it's not supporting more than one subject transaction.
	idToTx := seq2.FromMap(beef.Transactions)

	// inDegree will contain the number of transactions for which the given tx is a parent
	inDegree := seq2.CollectToMap(seq2.MapValues(idToTx, func(tx *transaction.BeefTx) int { return 0 }))

	// txsNotMined we are not interested in inputs of mined transactions
	txsNotMined := seq.Filter(seq2.Values(idToTx), func(tx *transaction.BeefTx) bool {
		return tx.Transaction.MerklePath == nil
	})

	inputs := seq.FlattenSlices(seq.Map(txsNotMined, func(tx *transaction.BeefTx) []*transaction.TransactionInput {
		return tx.Transaction.Inputs
	}))

	inputsIds := seq.Map(inputs, func(input *transaction.TransactionInput) chainhash.Hash {
		return *input.SourceTXID
	})

	seq.ForEach(inputsIds, func(inputTxID chainhash.Hash) {
		if _, ok := inDegree[inputTxID]; !ok {
			panic(fmt.Sprintf("unexpected input txid %s, this shouldn't ever happen", inputTxID))
		}
		inDegree[inputTxID]++
	})

	txIDsWithoutChildren := seq2.FilterByValue(seq2.FromMap(inDegree), is.Zero)

	subjectTxs := seq.Collect(seq2.Keys(txIDsWithoutChildren))
	if len(subjectTxs) != 1 {
		return "", fmt.Errorf("expected only one subject tx, but got %d", len(subjectTxs))
	}

	subjectTx, ok := beef.Transactions[subjectTxs[0]]
	if !ok {
		return "", fmt.Errorf("expected to find subject tx %s in beef, but it was not found, this shouldn't ever happen", subjectTxs[0])
	}

	// Another temporary workaround until go-sdk properly implements BEEF serialization
	tx, err := rebuildSubjectTx(subjectTx.Transaction, beef)
	if err != nil {
		return "", fmt.Errorf("failed to rebuild subject tx: %w", err)
	}

	beefHex, err := tx.BEEFHex()
	if err != nil {
		return "", fmt.Errorf("failed to convert subject tx into BEEF hex: %w", err)
	}
	return beefHex, nil
}

func (s *Service) bindBumpsAndTransactions(beef *transaction.Beef) {
	for i, bump := range beef.BUMPs {
		if len(bump.Path) == 0 || len(bump.Path[0]) == 0 {
			s.logger.Warn("got bump without bottom path", slog.String("merklePath", bump.Hex()))
			continue
		}
		for _, element := range bump.Path[0] {
			if element.Txid != nil && *element.Txid {
				if element.Hash == nil {
					s.logger.Error("got leaf marked as txid in BUMP but hash is nil")
					continue
				}
				tx, ok := beef.Transactions[*element.Hash]
				if !ok {
					s.logger.Warn("got leaf marked as txid in BUMP that is not part of the BEEF", slog.String("txid", element.Hash.String()))
					continue
				}
				tx.BumpIndex = i
				tx.DataFormat = transaction.RawTxAndBumpIndex
				tx.Transaction.MerklePath = bump
			}
		}
	}
}

func rebuildSubjectTx(tx *transaction.Transaction, beef *transaction.Beef) (*transaction.Transaction, error) {
	for _, input := range tx.Inputs {
		err := hydrateInput(input, beef, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to hydrate input %s of tx %s: %w", input.SourceTXID.String(), tx.TxID().String(), err)
		}
	}
	return tx, nil
}

func hydrateInput(input *transaction.TransactionInput, beef *transaction.Beef, depth int) error {
	txID := input.SourceTXID.String()
	if depth > 100 {
		return fmt.Errorf("could not hydrate the input %s: too many recursions", txID)
	}
	if input.SourceTransaction != nil {
		return nil
	}

	tx, ok := beef.Transactions[*input.SourceTXID]
	if !ok {
		return fmt.Errorf("could not find transaction %s in beef", txID)
	}
	input.SourceTransaction = tx.Transaction
	if tx.DataFormat == transaction.RawTxAndBumpIndex {
		if !is.Between(tx.BumpIndex, 0, len(beef.BUMPs)-1) {
			return fmt.Errorf("cannot find bump with index %d for tx %s", tx.BumpIndex, txID)
		}
		input.SourceTransaction.MerklePath = beef.BUMPs[tx.BumpIndex]
		return nil
	}
	for _, source := range input.SourceTransaction.Inputs {
		err := hydrateInput(source, beef, depth+1)
		if err != nil {
			return err
		}
	}
	return nil
}
