// Command perf_torture replicates the "1 desired change UTXO" torture scenario on mainnet:
// it runs N createActions (single 0-satoshi OP_FALSE OP_RETURN deadbeef output) back-to-back,
// broadcasting through Arcade, and tracks every transaction until it reaches MINED state.
//
// A net/http/pprof endpoint is exposed so the resulting flame graph can be compared with the
// one reported by the client (dominated by transaction.(*Beef).MergeTransaction recursion).
//
// Usage:
//  1. go run ./examples/perf_torture                  -> prints a funding address, exits
//  2. send funds to that address, note the txid
//  3. go run ./examples/perf_torture -txid <txid>     -> internalizes funding, runs the test
//     (later runs can omit -txid: balance is persisted in storage.sqlite)
//
// While it runs, capture a CPU profile with:
//
//	go tool pprof -http=:8080 "http://localhost:6060/debug/pprof/profile?seconds=30"
package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // bound to localhost; profiling is the whole point of this example
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	originator      = "perf-torture.example.com"
	baseLabel       = "perf-torture"
	changeBasket    = "default"
	fundingWaitMax  = 10 * time.Minute
	fundingWaitStep = 15 * time.Second
)

type runtimeConfig struct {
	PrivateKey       string `mapstructure:"private_key"`
	ServerPrivateKey string `mapstructure:"server_private_key"`
}

func defaultRuntimeConfig() runtimeConfig { return runtimeConfig{} }

type txRecord struct {
	Index           int
	TxID            string
	CreatedAt       time.Time
	CreateDuration  time.Duration
	BroadcastStatus string
	MinedAt         time.Time
	FinalStatus     string
}

func main() {
	var (
		n            = flag.Int("n", 100, "number of transactions to create")
		fundTxID     = flag.String("txid", "", "funding txid to internalize before running (send funds to the printed address first)")
		fundVout     = flag.Uint("vout", 0, "output index of the funding txid that pays the wallet's address")
		arcadeURL    = flag.String("arcade", "https://arcade-v2-us-1.bsvblockchain.tech", "Arcade broadcast endpoint")
		pprofAddr    = flag.String("pprof", "localhost:6060", "pprof listen address")
		interval     = flag.Duration("interval", 0, "delay between createActions (0 = as fast as possible)")
		minedTimeout = flag.Duration("mined-timeout", 4*time.Hour, "max time to wait for all transactions to reach MINED")
		pollEvery    = flag.Duration("poll", 15*time.Second, "status poll interval")
		feeSatKB     = flag.Int64("fee-satkb", 100, "fee model in sat/kB (100 matches production defaults; lower stretches funding)")
		listBEEF     = flag.Bool("list-beef", false, "after each createAction, call ListOutputs with entire transactions (replicates the client's listAllSpendableOutputs-per-operation pattern)")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("pprof listening", "addr", *pprofAddr,
			"capture", fmt.Sprintf("go tool pprof -http=:8080 \"http://%s/debug/pprof/profile?seconds=30\"", *pprofAddr))
		_ = http.ListenAndServe(*pprofAddr, nil) //nolint:gosec // localhost profiling endpoint
	}()

	cfg := loadOrGenerateConfig(logger)

	privateKey, err := ec.PrivateKeyFromHex(cfg.PrivateKey)
	if err != nil {
		fatal(logger, "invalid private key in config", err)
	}
	identityKey := privateKey.PubKey().ToDERHex()

	infraCfg := infra.Defaults() // mainnet defaults
	infraCfg.ServerPrivateKey = cfg.ServerPrivateKey
	infraCfg.ChangeBasket.NumberOfDesiredUTXOs = 1 // the torture condition
	infraCfg.FeeModel.Value = *feeSatKB
	infraCfg.DBConfig.SQLite.ConnectionString = filepath.Join(exampleDir(), "storage.sqlite")
	infraCfg.Services.Arcade.Enabled = true
	infraCfg.Services.Arcade.URL = *arcadeURL
	infraCfg.Services.Arcade.EventsURL = *arcadeURL

	activeServices := services.New(logger, infraCfg.Services)

	activeStorage, storageCleanup, err := createStorage(ctx, logger, &infraCfg, activeServices)
	if err != nil {
		fatal(logger, "failed to create storage", err)
	}
	defer storageCleanup()

	userWallet, err := wallet.NewWithStorageFactory(infraCfg.BSVNetwork, privateKey,
		func(_ sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
			return activeStorage, func() {}, nil
		})
	if err != nil {
		fatal(logger, "failed to create wallet", err)
	}
	defer userWallet.Close()

	// The build-time WithChangeBasket only seeds NEW users; force it for an existing user row too.
	if err := activeStorage.UpdateChangeBasket(ctx, identityKey, wdk.BasketConfiguration{
		NumberOfDesiredUTXOs:    1,
		MinimumDesiredUTXOValue: wdk.MinimumDesiredUTXOValueForChange,
	}); err != nil {
		logger.Warn("could not update change basket for existing user (fine on first run)", "error", err)
	}

	if *fundTxID != "" {
		if err := internalizeFunding(ctx, logger, activeServices, userWallet, *fundTxID, uint32(*fundVout)); err != nil {
			fatal(logger, "failed to internalize funding tx", err)
		}
	}

	balance, utxoCount := walletBalance(ctx, logger, userWallet)
	logger.Info("wallet state", "identity_key", identityKey, "balance_sats", balance, "utxos", utxoCount)

	if balance == 0 {
		printFundingInstructions(privateKey)
		return
	}

	runTortureTest(ctx, logger, userWallet, *n, *interval, *minedTimeout, *pollEvery, *listBEEF)
}

// ---------------------------------------------------------------------------
// setup
// ---------------------------------------------------------------------------

func exampleDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to resolve example directory")
	}
	return filepath.Dir(filename)
}

func configFilePath() string { return filepath.Join(exampleDir(), "perf-torture-config.yaml") }

func loadOrGenerateConfig(logger *slog.Logger) runtimeConfig {
	path := configFilePath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		userKey, err := ec.NewPrivateKey()
		if err != nil {
			fatal(logger, "failed to generate user key", err)
		}
		serverKey, err := ec.NewPrivateKey()
		if err != nil {
			fatal(logger, "failed to generate server key", err)
		}

		cfg := runtimeConfig{
			PrivateKey:       hex.EncodeToString(userKey.Serialize()),
			ServerPrivateKey: hex.EncodeToString(serverKey.Serialize()),
		}
		if err := config.ToYAMLFile(cfg, path); err != nil {
			fatal(logger, "failed to write generated config", err)
		}
		logger.Info("generated new config with random keys", "path", path)
		return cfg
	}

	loader := config.NewLoader(defaultRuntimeConfig, "PERF_TORTURE")
	if err := loader.SetConfigFilePath(path); err != nil {
		fatal(logger, "failed to set config path", err)
	}
	cfg, err := loader.Load()
	if err != nil {
		fatal(logger, "failed to load config", err)
	}
	return cfg
}

func createStorage(ctx context.Context, logger *slog.Logger, cfg *infra.Config, activeServices *services.WalletServices) (*storage.Provider, func(), error) {
	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive storage identity key: %w", err)
	}

	options := append(
		infra.GORMProviderOptionsFromConfig(cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, options...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage: %w", err)
	}

	if _, err = activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey); err != nil {
		return nil, nil, fmt.Errorf("failed to migrate storage: %w", err)
	}

	var daemon *monitor.Daemon
	if cfg.Monitor.Enabled {
		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create monitor daemon: %w", err)
		}
		if err = daemon.Start(ctx, cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			return nil, nil, fmt.Errorf("failed to start monitor daemon: %w", err)
		}
	}

	cleanup := func() {
		if daemon != nil {
			if err := daemon.Stop(); err != nil {
				logger.Error("failed to stop monitor daemon", "error", err)
			}
		}
		activeStorage.Stop()
	}
	return activeStorage, cleanup, nil
}

// ---------------------------------------------------------------------------
// funding
// ---------------------------------------------------------------------------

func printFundingInstructions(privateKey *ec.PrivateKey) {
	parts := utils.DerivationParts()
	keyID := brc29.KeyID{
		DerivationPrefix: base64.StdEncoding.EncodeToString(parts.DerivationPrefix),
		DerivationSuffix: base64.StdEncoding.EncodeToString(parts.DerivationSuffix),
	}
	address, err := brc29.AddressForSelf(parts.SenderIdentityKey, keyID, privateKey, brc29.WithMainNet())
	if err != nil {
		panic(fmt.Errorf("failed to derive funding address: %w", err))
	}

	fmt.Println()
	fmt.Println("========================================================================")
	fmt.Println(" Wallet is empty. Send mainnet funds (a few thousand sats is plenty) to:")
	fmt.Println()
	fmt.Printf("    %s\n", address.AddressString)
	fmt.Println()
	fmt.Println(" Then internalize the funding transaction and start the test with:")
	fmt.Println()
	fmt.Println("    go run ./examples/perf_torture -txid <funding-txid>")
	fmt.Println("========================================================================")
	fmt.Println()
}

func internalizeFunding(ctx context.Context, logger *slog.Logger, srv *services.WalletServices, userWallet *wallet.Wallet, txID string, vout uint32) error {
	txIDHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return fmt.Errorf("invalid funding txid %q: %w", txID, err)
	}

	// The tx may take a while to be visible to (or mined by) the backing services; retry.
	var beefBytes []byte
	deadline := time.Now().Add(fundingWaitMax)
	for {
		beef, err := srv.GetBEEF(ctx, txID, nil)
		if err == nil {
			beefBytes, err = beef.AtomicBytes(txIDHash)
			if err != nil {
				return fmt.Errorf("failed to serialize atomic BEEF: %w", err)
			}
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("gave up fetching BEEF for funding tx %s: %w", txID, err)
		}
		logger.Info("funding tx not yet retrievable, waiting", "txid", txID, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(fundingWaitStep):
		}
	}

	args := sdk.InternalizeActionArgs{
		Tx: beefBytes,
		Outputs: []sdk.InternalizeOutput{{
			OutputIndex:       vout,
			Protocol:          "wallet payment",
			PaymentRemittance: utils.DerivationParts(),
		}},
		Description: "perf torture funding",
	}
	if _, err := userWallet.InternalizeAction(ctx, args, originator); err != nil {
		return fmt.Errorf("internalize action failed: %w", err)
	}
	logger.Info("funding internalized", "txid", txID)
	return nil
}

func walletBalance(ctx context.Context, logger *slog.Logger, userWallet *wallet.Wallet) (balance uint64, count uint32) {
	limit := uint32(100)
	var offset uint32
	for {
		outputs, err := userWallet.ListOutputs(ctx, sdk.ListOutputsArgs{
			Basket: changeBasket,
			Limit:  &limit,
			Offset: &offset,
		}, originator)
		if err != nil {
			fatal(logger, "failed to list outputs", err)
		}
		for _, output := range outputs.Outputs {
			balance += output.Satoshis
		}
		count = outputs.TotalOutputs
		offset += uint32(len(outputs.Outputs))
		if offset >= outputs.TotalOutputs || len(outputs.Outputs) == 0 {
			break
		}
	}
	return balance, count
}

// ---------------------------------------------------------------------------
// the torture loop
// ---------------------------------------------------------------------------

func opReturnDeadbeef() []byte {
	s := &script.Script{}
	if err := s.AppendOpcodes(script.OpFALSE, script.OpRETURN); err != nil {
		panic(err)
	}
	if err := s.AppendPushData([]byte{0xde, 0xad, 0xbe, 0xef}); err != nil {
		panic(err)
	}
	return s.Bytes()
}

func runTortureTest(ctx context.Context, logger *slog.Logger, userWallet *wallet.Wallet, n int, interval, minedTimeout, pollEvery time.Duration, listBEEF bool) {
	runLabel := fmt.Sprintf("%s-%d", baseLabel, time.Now().Unix())
	lockingScript := opReturnDeadbeef()
	records := make([]*txRecord, 0, n)
	start := time.Now()

	logger.Info("starting torture run", "n", n, "label", runLabel, "interval", interval.String())

	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			logger.Warn("interrupted during creation", "created", len(records))
			break
		}

		createArgs := sdk.CreateActionArgs{
			Description: fmt.Sprintf("perf torture %d/%d", i+1, n),
			Outputs: []sdk.CreateActionOutput{{
				LockingScript:     lockingScript,
				Satoshis:          0,
				OutputDescription: "OP_FALSE OP_RETURN deadbeef",
			}},
			Labels: []string{baseLabel, runLabel},
			Options: &sdk.CreateActionOptions{
				AcceptDelayedBroadcast: to.Ptr(false),
			},
		}

		attempt := 0
		for {
			attempt++
			createStart := time.Now()
			result, err := userWallet.CreateAction(ctx, createArgs, originator)
			createDur := time.Since(createStart)

			if err != nil {
				logger.Error("createAction failed", "index", i+1, "attempt", attempt, "took", createDur.String(), "error", err)
				if ctx.Err() != nil {
					break
				}
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
				}
				continue
			}

			rec := &txRecord{
				Index:          i + 1,
				TxID:           result.Txid.String(),
				CreatedAt:      createStart,
				CreateDuration: createDur,
			}
			if len(result.SendWithResults) > 0 {
				rec.BroadcastStatus = string(result.SendWithResults[0].Status)
			}
			records = append(records, rec)
			logger.Info("created", "index", i+1, "txid", rec.TxID, "took", createDur.String(), "broadcast", rec.BroadcastStatus)
			break
		}

		if listBEEF {
			listStart := time.Now()
			limit := uint32(100)
			_, err := userWallet.ListOutputs(ctx, sdk.ListOutputsArgs{
				Basket:  changeBasket,
				Include: sdk.OutputIncludeEntireTransactions,
				Limit:   &limit,
			}, originator)
			if err != nil {
				logger.Error("listOutputs with BEEF failed", "index", i+1, "error", err)
			} else if (i+1)%25 == 0 {
				logger.Info("listOutputs with BEEF", "index", i+1, "took", time.Since(listStart).String())
			}
		}

		if interval > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(interval):
			}
		}
	}

	logger.Info("creation phase done", "created", len(records), "elapsed", time.Since(start).String())

	waitForMined(ctx, logger, userWallet, runLabel, records, minedTimeout, pollEvery)
	writeResults(logger, runLabel, records, start)
}

func waitForMined(ctx context.Context, logger *slog.Logger, userWallet *wallet.Wallet, runLabel string, records []*txRecord, minedTimeout, pollEvery time.Duration) {
	if len(records) == 0 {
		return
	}
	byTxID := make(map[string]*txRecord, len(records))
	for _, r := range records {
		byTxID[r.TxID] = r
	}

	deadline := time.Now().Add(minedTimeout)
	for {
		if ctx.Err() != nil || time.Now().After(deadline) {
			logger.Warn("stopped waiting for MINED", "reason", "timeout or interrupt")
			return
		}

		mined, pending := pollStatuses(ctx, logger, userWallet, runLabel, byTxID)

		_, utxoCount := walletBalance(ctx, logger, userWallet)
		logger.Info("status", "mined", mined, "pending", pending,
			"failed_or_aborted", len(byTxID)-mined-pending, "utxos", utxoCount)

		if mined+len(failedRecords(records)) >= len(records) {
			logger.Info("all transactions reached a terminal state")
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollEvery):
		}
	}
}

func pollStatuses(ctx context.Context, logger *slog.Logger, userWallet *wallet.Wallet, runLabel string, byTxID map[string]*txRecord) (mined, pending int) {
	limit := uint32(10000)

	// Non-terminal + completed statuses (default filter excludes failed/aborted).
	list, err := userWallet.ListActions(ctx, sdk.ListActionsArgs{
		Labels: []string{runLabel},
		Limit:  &limit,
	}, originator)
	if err != nil {
		logger.Error("listActions failed", "error", err)
		return 0, len(byTxID)
	}

	now := time.Now()
	for _, action := range list.Actions {
		rec, ok := byTxID[action.Txid.String()]
		if !ok {
			continue
		}
		status := string(action.Status)
		rec.FinalStatus = status
		if isMinedStatus(status) {
			if rec.MinedAt.IsZero() {
				rec.MinedAt = now
			}
			mined++
		} else {
			pending++
		}
	}

	// Failed/aborted (the "failed" label is a spec-op that flips the status filter).
	failed, err := userWallet.ListActions(ctx, sdk.ListActionsArgs{
		Labels: []string{"failed"},
		Limit:  &limit,
	}, originator)
	if err == nil {
		for _, action := range failed.Actions {
			if rec, ok := byTxID[action.Txid.String()]; ok {
				rec.FinalStatus = string(action.Status)
			}
		}
	}

	return mined, pending
}

func isMinedStatus(status string) bool {
	return status == "completed" || status == "mined"
}

func isFailedStatus(status string) bool {
	return status == "failed" || status == "aborted"
}

func failedRecords(records []*txRecord) []*txRecord {
	var out []*txRecord
	for _, r := range records {
		if isFailedStatus(r.FinalStatus) {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// results
// ---------------------------------------------------------------------------

func writeResults(logger *slog.Logger, runLabel string, records []*txRecord, start time.Time) {
	if len(records) == 0 {
		logger.Warn("no records to write")
		return
	}

	path := filepath.Join(exampleDir(), fmt.Sprintf("results-%s.csv", runLabel))
	f, err := os.Create(path) //nolint:gosec // path is derived from source location
	if err != nil {
		logger.Error("failed to create results file", "error", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"index", "txid", "created_at", "create_ms", "broadcast_status", "mined_at", "secs_to_mined", "final_status"})

	var createDurs, minedDurs []time.Duration
	minedCount, failedCount := 0, 0

	for _, r := range records {
		minedAt, secsToMined := "", ""
		if !r.MinedAt.IsZero() {
			minedAt = r.MinedAt.Format(time.RFC3339)
			d := r.MinedAt.Sub(r.CreatedAt)
			secsToMined = fmt.Sprintf("%.0f", d.Seconds())
			minedDurs = append(minedDurs, d)
			minedCount++
		}
		if isFailedStatus(r.FinalStatus) {
			failedCount++
		}
		createDurs = append(createDurs, r.CreateDuration)

		_ = w.Write([]string{
			fmt.Sprintf("%d", r.Index),
			r.TxID,
			r.CreatedAt.Format(time.RFC3339),
			fmt.Sprintf("%d", r.CreateDuration.Milliseconds()),
			r.BroadcastStatus,
			minedAt,
			secsToMined,
			r.FinalStatus,
		})
	}

	elapsed := time.Since(start)
	logger.Info("run summary",
		"created", len(records),
		"mined", minedCount,
		"failed_or_aborted", failedCount,
		"elapsed", elapsed.String(),
		"create_p50", percentile(createDurs, 50).String(),
		"create_p95", percentile(createDurs, 95).String(),
		"create_max", percentile(createDurs, 100).String(),
		"to_mined_p50", percentile(minedDurs, 50).String(),
		"to_mined_p95", percentile(minedDurs, 95).String(),
		"results_csv", path,
	)
}

func percentile(durs []time.Duration, p int) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (len(sorted)*p + 99) / 100
	if idx > 0 {
		idx--
	}
	return sorted[idx]
}

func fatal(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "error", err)
	os.Exit(1)
}
