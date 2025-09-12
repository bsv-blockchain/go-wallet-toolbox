package services_test

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"
)

var mockTxID = testvectors.GivenTX().WithInput(10).WithP2PKHOutput(9).ID().String()

func TestServicesConfig_CustomServiceImplementation(t *testing.T) {
	given := testservices.GivenServices(t)
	counter := 0

	// and:
	customImplementation := services.Implementation{
		RawTx: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
			counter++
			return &wdk.RawTxResult{}, nil
		},
		PostBEEF: func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
			counter++
			return &wdk.PostedBEEF{}, nil
		},
		MerklePath: func(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
			counter++
			return &wdk.MerklePathResult{}, nil
		},
		FindChainTipHeader: func(ctx context.Context) (*wdk.ChainBlockHeader, error) {
			counter++
			return &wdk.ChainBlockHeader{}, nil
		},
		IsValidRootForHeight: func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
			counter++
			return true, nil
		},
		CurrentHeight: func(ctx context.Context) (uint32, error) {
			counter++
			return 0, nil
		},
		GetScriptHashHistory: func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
			counter++
			return &wdk.ScriptHistoryResult{}, nil
		},
		HashToHeader: func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
			counter++
			return &wdk.ChainBlockHeader{}, nil
		},
		ChainHeaderByHeight: func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error) {
			counter++
			return &wdk.ChainBaseBlockHeader{}, nil
		},
		GetStatusForTxIDs: func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
			counter++
			return &wdk.GetStatusForTxIDsResult{}, nil
		},
		GetUtxoStatus: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
			counter++
			return &wdk.UtxoStatusResult{}, nil
		},
		IsUtxo: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error) {
			counter++
			return true, nil
		},
		BsvExchangeRate: func(ctx context.Context) (float64, error) {
			counter++
			return 0, nil
		},
	}

	// and:
	service := given.Services().
		Opts(services.WithCustomImplementation("custom", customImplementation)).
		New()

	// when:
	_, _ = service.RawTx(t.Context(), mockTxID)
	_, _ = service.PostBEEF(t.Context(), &transaction.Beef{}, []string{mockTxID})
	_, _ = service.MerklePath(t.Context(), mockTxID)
	_, _ = service.FindChainTipHeader(t.Context())
	_, _ = service.IsValidRootForHeight(t.Context(), &chainhash.Hash{}, 0)
	_, _ = service.CurrentHeight(t.Context())
	_, _ = service.GetScriptHashHistory(t.Context(), "scriptHash")
	_, _ = service.HashToHeader(t.Context(), "hash")
	_, _ = service.ChainHeaderByHeight(t.Context(), 0)
	_, _ = service.GetStatusForTxIDs(t.Context(), []string{mockTxID})
	_, _ = service.GetUtxoStatus(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.IsUtxo(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.BsvExchangeRate(t.Context())

	// then:
	require.Equal(t, 13, counter)
}

func TestServicesConfig_UseModifiers(t *testing.T) {
	given := testservices.GivenServices(t)
	counter := 0

	// and:
	opts := []func(option *services.Options){
		services.WithRawTxMethodsModifier(func(original []services.Named[services.RawTxFunc]) []services.Named[services.RawTxFunc] {
			return append([]services.Named[services.RawTxFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
					counter++
					return &wdk.RawTxResult{}, nil
				},
			}}, original...)
		}),
		services.WithPostBEEFMethodsModifier(func(original []services.Named[services.PostBEEFFunc]) []services.Named[services.PostBEEFFunc] {
			return append([]services.Named[services.PostBEEFFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
					counter++
					return &wdk.PostedBEEF{}, nil
				},
			}}, original...)
		}),
		services.WithMerklePathMethodsModifier(func(original []services.Named[services.MerklePathFunc]) []services.Named[services.MerklePathFunc] {
			return append([]services.Named[services.MerklePathFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
					counter++
					return &wdk.MerklePathResult{}, nil
				},
			}}, original...)
		}),
		services.WithFindChainTipHeaderMethodsModifier(func(original []services.Named[services.FindChainTipHeaderFunc]) []services.Named[services.FindChainTipHeaderFunc] {
			return append([]services.Named[services.FindChainTipHeaderFunc]{{
				Name: "custom",
				Item: func(ctx context.Context) (*wdk.ChainBlockHeader, error) {
					counter++
					return &wdk.ChainBlockHeader{}, nil
				},
			}}, original...)
		}),
		services.WithIsValidRootForHeightMethodsModifier(func(original []services.Named[services.IsValidRootForHeightFunc]) []services.Named[services.IsValidRootForHeightFunc] {
			return append([]services.Named[services.IsValidRootForHeightFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
					counter++
					return true, nil
				},
			}}, original...)
		}),
		services.WithCurrentHeightMethodsModifier(func(original []services.Named[services.CurrentHeightFunc]) []services.Named[services.CurrentHeightFunc] {
			return append([]services.Named[services.CurrentHeightFunc]{{
				Name: "custom",
				Item: func(ctx context.Context) (uint32, error) {
					counter++
					return 0, nil
				},
			}}, original...)
		}),
		services.WithGetScriptHashHistoryMethodsModifier(func(original []services.Named[services.GetScriptHashHistoryFunc]) []services.Named[services.GetScriptHashHistoryFunc] {
			return append([]services.Named[services.GetScriptHashHistoryFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
					counter++
					return &wdk.ScriptHistoryResult{}, nil
				},
			}}, original...)
		}),
		services.WithHashToHeaderMethodsModifier(func(original []services.Named[services.HashToHeaderFunc]) []services.Named[services.HashToHeaderFunc] {
			return append([]services.Named[services.HashToHeaderFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
					counter++
					return &wdk.ChainBlockHeader{}, nil
				},
			}}, original...)
		}),
		services.WithChainHeaderByHeightMethodsModifier(func(original []services.Named[services.ChainHeaderByHeightFunc]) []services.Named[services.ChainHeaderByHeightFunc] {
			return append([]services.Named[services.ChainHeaderByHeightFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error) {
					counter++
					return &wdk.ChainBaseBlockHeader{}, nil
				},
			}}, original...)
		}),
		services.WithGetStatusForTxIDsMethodsModifier(func(original []services.Named[services.GetStatusForTxIDsFunc]) []services.Named[services.GetStatusForTxIDsFunc] {
			return append([]services.Named[services.GetStatusForTxIDsFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
					counter++
					return &wdk.GetStatusForTxIDsResult{}, nil
				},
			}}, original...)
		}),
		services.WithGetUtxoStatusMethodsModifier(func(original []services.Named[services.GetUtxoStatusFunc]) []services.Named[services.GetUtxoStatusFunc] {
			return append([]services.Named[services.GetUtxoStatusFunc]{{
				Name: "custom",
				Item: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
					counter++
					return &wdk.UtxoStatusResult{}, nil
				},
			}}, original...)
		}),
		services.WithIsUtxoMethodsModifier(func(original []services.Named[services.IsUtxo]) []services.Named[services.IsUtxo] {
			return append([]services.Named[services.IsUtxo]{{
				Name: "custom",
				Item: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error) {
					counter++
					return true, nil
				},
			}}, original...)
		}),
		services.WithBsvExchangeRateMethodsModifier(func(original []services.Named[services.BsvExchangeRateFunc]) []services.Named[services.BsvExchangeRateFunc] {
			return append([]services.Named[services.BsvExchangeRateFunc]{{
				Name: "custom",
				Item: func(ctx context.Context) (float64, error) {
					counter++
					return 0, nil
				},
			}}, original...)
		}),
	}

	// and:
	service := given.Services().
		Opts(opts...).
		New()

	// when:
	_, _ = service.RawTx(t.Context(), mockTxID)
	_, _ = service.PostBEEF(t.Context(), &transaction.Beef{}, []string{mockTxID})
	_, _ = service.MerklePath(t.Context(), mockTxID)
	_, _ = service.FindChainTipHeader(t.Context())
	_, _ = service.IsValidRootForHeight(t.Context(), &chainhash.Hash{}, 0)
	_, _ = service.CurrentHeight(t.Context())
	_, _ = service.GetScriptHashHistory(t.Context(), "scriptHash")
	_, _ = service.HashToHeader(t.Context(), "hash")
	_, _ = service.ChainHeaderByHeight(t.Context(), 0)
	_, _ = service.GetStatusForTxIDs(t.Context(), []string{mockTxID})
	_, _ = service.GetUtxoStatus(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.IsUtxo(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.BsvExchangeRate(t.Context())

	// then:
	require.Equal(t, 13, counter)
}

func TestServicesConfig_ProvideImplementationWithTheSameName(t *testing.T) {
	// given:
	given := testservices.GivenServices(t)
	counter1 := 0
	counter2 := 0
	const theSameName = "custom"

	// and:
	customImplementation1 := services.Implementation{
		RawTx: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
			counter1++
			return &wdk.RawTxResult{}, nil
		},
	}

	customImplementation2 := services.Implementation{
		RawTx: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
			counter2 += 1
			return &wdk.RawTxResult{}, nil
		},
	}

	// and:
	service := given.Services().
		Opts(
			services.WithCustomImplementation(theSameName, customImplementation1),
			services.WithCustomImplementation(theSameName, customImplementation2),
		).
		New()

	// when:
	_, err := service.RawTx(t.Context(), mockTxID)

	// then:
	require.NoError(t, err)
	require.Equal(t, 1, counter1)
	require.Equal(t, 0, counter2)
}

func TestServicesConfig_DisableAllPredefinedServices(t *testing.T) {
	given := testservices.GivenServices(t)
	const errContent = "no services registered"

	// and:
	service := given.Services().
		Config(
			testservices.WithEnabledARC(false),
			testservices.WithEnabledBHS(false),
			testservices.WithEnabledWoC(false),
			testservices.WithEnabledBitails(false),
		).
		New()

	// when:
	_, err := service.RawTx(t.Context(), mockTxID)
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.PostBEEF(t.Context(), &transaction.Beef{}, []string{mockTxID})
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.MerklePath(t.Context(), mockTxID)
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.FindChainTipHeader(t.Context())
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.IsValidRootForHeight(t.Context(), &chainhash.Hash{}, 0)
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.CurrentHeight(t.Context())
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.GetScriptHashHistory(t.Context(), "scriptHash")
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.HashToHeader(t.Context(), "hash")
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.ChainHeaderByHeight(t.Context(), 0)
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.GetStatusForTxIDs(t.Context(), []string{mockTxID})
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.GetUtxoStatus(t.Context(), "scriptHash", &transaction.Outpoint{})
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.IsUtxo(t.Context(), "scriptHash", &transaction.Outpoint{})
	// then:
	require.ErrorContains(t, err, errContent)

	// when:
	_, err = service.BsvExchangeRate(t.Context())
	// then:
	require.ErrorContains(t, err, errContent)
}
