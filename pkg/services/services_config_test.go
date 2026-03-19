package services_test

import (
	"context"
	"slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var (
	mockTx   = testvectors.GivenTX().WithInput(10).WithP2PKHOutput(9)
	mockTxID = mockTx.ID().String()
)

func TestServicesConfig_CustomServiceImplementation(t *testing.T) {
	// given:
	given := testservices.GivenServices(t)
	mock := &mockImplementation{}

	// and:
	customImplementation := services.ToImplementation(mock)

	// and:
	service := given.Services().
		Opts(services.WithCustomImplementation("custom", customImplementation)).
		New()

	// when:
	callAllMethods(t, service)

	// then:
	mock.allCalled(t)
}

func TestServicesConfig_CustomServicePartialImplementation(t *testing.T) {
	// given:
	given := testservices.GivenServices(t)
	mock := &mockPartialRawTxImplementation{}

	// and:
	customImplementation := services.ToImplementation(mock)

	// and:
	service := given.Services().
		Opts(services.WithCustomImplementation("custom", customImplementation)).
		New()

	// when:
	_, _ = service.RawTx(t.Context(), mockTxID)
	// then:
	assert.NotZero(t, mock.rawTxCounter)

	// when:
	given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "USD"
		}`, nil)
	rate, err := service.BsvExchangeRate(t.Context())

	// then:
	require.NoError(t, err)
	require.InDelta(t, 50.5, rate, 0.001)
}

func TestServicesConfig_UseModifiers(t *testing.T) {
	given := testservices.GivenServices(t)

	mock := &mockImplementation{}

	// and:
	opts := []func(option *services.Options){
		services.WithRawTxMethodsModifier(func(original []services.Named[services.RawTxFunc]) []services.Named[services.RawTxFunc] {
			return append([]services.Named[services.RawTxFunc]{{
				Name: "custom",
				Item: mock.RawTx,
			}}, original...)
		}),
		services.WithPostEFMethodsModifier(func(original []services.Named[services.PostEFFunc]) []services.Named[services.PostEFFunc] {
			return append([]services.Named[services.PostEFFunc]{{
				Name: "custom",
				Item: mock.PostEF,
			}}, original...)
		}),
		services.WithPostTXMethodsModifier(func(original []services.Named[services.PostTXFunc]) []services.Named[services.PostTXFunc] {
			return append([]services.Named[services.PostTXFunc]{{
				Name: "custom",
				Item: mock.PostTX,
			}}, original...)
		}),
		services.WithMerklePathMethodsModifier(func(original []services.Named[services.MerklePathFunc]) []services.Named[services.MerklePathFunc] {
			return append([]services.Named[services.MerklePathFunc]{{
				Name: "custom",
				Item: mock.MerklePath,
			}}, original...)
		}),
		services.WithFindChainTipHeaderMethodsModifier(func(original []services.Named[services.FindChainTipHeaderFunc]) []services.Named[services.FindChainTipHeaderFunc] {
			return append([]services.Named[services.FindChainTipHeaderFunc]{{
				Name: "custom",
				Item: mock.FindChainTipHeader,
			}}, original...)
		}),
		services.WithIsValidRootForHeightMethodsModifier(func(original []services.Named[services.IsValidRootForHeightFunc]) []services.Named[services.IsValidRootForHeightFunc] {
			return append([]services.Named[services.IsValidRootForHeightFunc]{{
				Name: "custom",
				Item: mock.IsValidRootForHeight,
			}}, original...)
		}),
		services.WithCurrentHeightMethodsModifier(func(original []services.Named[services.CurrentHeightFunc]) []services.Named[services.CurrentHeightFunc] {
			return append([]services.Named[services.CurrentHeightFunc]{{
				Name: "custom",
				Item: mock.CurrentHeight,
			}}, original...)
		}),
		services.WithGetScriptHashHistoryMethodsModifier(func(original []services.Named[services.GetScriptHashHistoryFunc]) []services.Named[services.GetScriptHashHistoryFunc] {
			return append([]services.Named[services.GetScriptHashHistoryFunc]{{
				Name: "custom",
				Item: mock.GetScriptHashHistory,
			}}, original...)
		}),
		services.WithHashToHeaderMethodsModifier(func(original []services.Named[services.HashToHeaderFunc]) []services.Named[services.HashToHeaderFunc] {
			return append([]services.Named[services.HashToHeaderFunc]{{
				Name: "custom",
				Item: mock.HashToHeader,
			}}, original...)
		}),
		services.WithChainHeaderByHeightMethodsModifier(func(original []services.Named[services.ChainHeaderByHeightFunc]) []services.Named[services.ChainHeaderByHeightFunc] {
			return append([]services.Named[services.ChainHeaderByHeightFunc]{{
				Name: "custom",
				Item: mock.ChainHeaderByHeight,
			}}, original...)
		}),
		services.WithGetStatusForTxIDsMethodsModifier(func(original []services.Named[services.GetStatusForTxIDsFunc]) []services.Named[services.GetStatusForTxIDsFunc] {
			return append([]services.Named[services.GetStatusForTxIDsFunc]{{
				Name: "custom",
				Item: mock.GetStatusForTxIDs,
			}}, original...)
		}),
		services.WithGetUtxoStatusMethodsModifier(func(original []services.Named[services.GetUtxoStatusFunc]) []services.Named[services.GetUtxoStatusFunc] {
			return append([]services.Named[services.GetUtxoStatusFunc]{{
				Name: "custom",
				Item: mock.GetUtxoStatus,
			}}, original...)
		}),
		services.WithIsUtxoMethodsModifier(func(original []services.Named[services.IsUtxo]) []services.Named[services.IsUtxo] {
			return append([]services.Named[services.IsUtxo]{{
				Name: "custom",
				Item: mock.IsUtxo,
			}}, original...)
		}),
		services.WithBsvExchangeRateMethodsModifier(func(original []services.Named[services.BsvExchangeRateFunc]) []services.Named[services.BsvExchangeRateFunc] {
			return append([]services.Named[services.BsvExchangeRateFunc]{{
				Name: "custom",
				Item: mock.BsvExchangeRate,
			}}, original...)
		}),
	}

	// and:
	service := given.Services().
		Opts(opts...).
		New()

	// when:
	callAllMethods(t, service)

	// then:
	mock.allCalled(t)
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
	errs := callAllMethods(t, service)

	// then:
	for _, err := range errs {
		require.ErrorContains(t, err, errContent)
	}
}

func TestServicesConfig_AdjustOrderOfServices(t *testing.T) {
	const customServiceName = "custom"
	const customServiceRate = 10.0

	t.Run("custom service is moved to the last position", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		customServiceCalled := 0

		// and:
		customImplementation := services.Implementation{
			BsvExchangeRate: func(ctx context.Context) (float64, error) {
				customServiceCalled++
				return customServiceRate, nil
			},
		}

		// and:
		service := given.Services().
			Opts(
				services.WithCustomImplementation(customServiceName, customImplementation),
				services.WithBsvExchangeRateMethodsModifier(func(original []services.Named[services.BsvExchangeRateFunc]) []services.Named[services.BsvExchangeRateFunc] {
					preferredOrder := []string{whatsonchain.ServiceName, customServiceName}
					slices.SortFunc(original, func(a, b services.Named[services.BsvExchangeRateFunc]) int {
						indexA := slices.Index(preferredOrder, a.Name)
						indexB := slices.Index(preferredOrder, b.Name)
						return indexA - indexB
					})
					return original
				}),
			).
			New()

		// when:
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "USD"
		}`, nil)
		rate, err := service.BsvExchangeRate(t.Context())

		// then:
		require.NoError(t, err)
		require.InDelta(t, 50.5, rate, 0.001)
		require.Equal(t, 0, customServiceCalled)
	})

	t.Run("custom service is preferred over WhatsOnChain, when no modifier applied - the tests proves that", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		customServiceCalled := 0

		// and:
		customImplementation := services.Implementation{
			BsvExchangeRate: func(ctx context.Context) (float64, error) {
				customServiceCalled++
				return customServiceRate, nil
			},
		}

		// and:
		service := given.Services().
			Opts(
				services.WithCustomImplementation(customServiceName, customImplementation),
			).
			New()

		// when:
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "USD"
		}`, nil)
		rate, err := service.BsvExchangeRate(t.Context())

		// then:
		require.NoError(t, err)
		require.InDelta(t, customServiceRate, rate, 0.001)
		require.Equal(t, 1, customServiceCalled)
	})
}

func TestServicesConfig_ToImplementation(t *testing.T) {
	// given:
	mock := &mockImplementation{}
	impl := services.ToImplementation(mock)

	// when:
	_, err := impl.RawTx(context.Background(), "txID")
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.PostEF(context.Background(), "efHex", "txID")
	// then:
	require.NoError(t, err)
	//
	// when:
	_, err = impl.PostTX(context.Background(), []byte{})
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.MerklePath(context.Background(), "txID")
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.FindChainTipHeader(context.Background())
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.IsValidRootForHeight(context.Background(), &chainhash.Hash{}, 0)
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.CurrentHeight(context.Background())
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.GetScriptHashHistory(context.Background(), "scriptHash")
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.HashToHeader(context.Background(), "hash")
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.ChainHeaderByHeight(context.Background(), 0)
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.GetStatusForTxIDs(context.Background(), []string{"txID"})
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.GetUtxoStatus(context.Background(), "scriptHash", &transaction.Outpoint{})
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.IsUtxo(context.Background(), "scriptHash", &transaction.Outpoint{})
	// then:
	require.NoError(t, err)

	// when:
	_, err = impl.BsvExchangeRate(context.Background())
	// then:
	require.NoError(t, err)

	// and:
	mock.allCalled(t)
}

type mockPartialRawTxImplementation struct {
	rawTxCounter int
}

func (m *mockPartialRawTxImplementation) RawTx(context.Context, string) (*wdk.RawTxResult, error) {
	m.rawTxCounter++
	return &wdk.RawTxResult{}, nil
}

type mockImplementation struct {
	mockPartialRawTxImplementation

	postEFCounter               int
	postTXCounter               int
	merklePathCounter           int
	findChainTipHeaderCounter   int
	isValidRootForHeightCounter int
	currentHeightCounter        int
	getScriptHashHistoryCounter int
	hashToHeaderCounter         int
	chainHeaderByHeightCounter  int
	getStatusForTxIDsCounter    int
	getUtxoStatusCounter        int
	isUtxoCounter               int
	bsvExchangeRateCounter      int
}

func (m *mockImplementation) PostEF(context.Context, string, string) (*wdk.PostedTxID, error) {
	m.postEFCounter++
	return &wdk.PostedTxID{}, nil
}

func (m *mockImplementation) PostTX(context.Context, []byte) (*wdk.PostedTxID, error) {
	m.postTXCounter++
	return &wdk.PostedTxID{}, nil
}

func (m *mockImplementation) MerklePath(context.Context, string) (*wdk.MerklePathResult, error) {
	m.merklePathCounter++
	return &wdk.MerklePathResult{}, nil
}

func (m *mockImplementation) FindChainTipHeader(context.Context) (*wdk.ChainBlockHeader, error) {
	m.findChainTipHeaderCounter++
	return &wdk.ChainBlockHeader{}, nil
}

func (m *mockImplementation) IsValidRootForHeight(context.Context, *chainhash.Hash, uint32) (bool, error) {
	m.isValidRootForHeightCounter++
	return true, nil
}

func (m *mockImplementation) CurrentHeight(context.Context) (uint32, error) {
	m.currentHeightCounter++
	return 0, nil
}

func (m *mockImplementation) GetScriptHashHistory(context.Context, string) (*wdk.ScriptHistoryResult, error) {
	m.getScriptHashHistoryCounter++
	return &wdk.ScriptHistoryResult{}, nil
}

func (m *mockImplementation) HashToHeader(context.Context, string) (*wdk.ChainBlockHeader, error) {
	m.hashToHeaderCounter++
	return &wdk.ChainBlockHeader{}, nil
}

func (m *mockImplementation) ChainHeaderByHeight(context.Context, uint32) (*wdk.ChainBlockHeader, error) {
	m.chainHeaderByHeightCounter++
	return &wdk.ChainBlockHeader{}, nil
}

func (m *mockImplementation) GetStatusForTxIDs(context.Context, []string) (*wdk.GetStatusForTxIDsResult, error) {
	m.getStatusForTxIDsCounter++
	return &wdk.GetStatusForTxIDsResult{}, nil
}

func (m *mockImplementation) GetUtxoStatus(context.Context, string, *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
	m.getUtxoStatusCounter++
	return &wdk.UtxoStatusResult{}, nil
}

func (m *mockImplementation) IsUtxo(context.Context, string, *transaction.Outpoint) (bool, error) {
	m.isUtxoCounter++
	return true, nil
}

func (m *mockImplementation) BsvExchangeRate(context.Context) (float64, error) {
	m.bsvExchangeRateCounter++
	return 0, nil
}

func (m *mockImplementation) OtherMethod() {
	// This method is intentionally created to demonstrate that
	// only the methods defined in the Implementation struct are considered.
}

func (m *mockImplementation) allCalled(t testing.TB) {
	assert.NotZero(t, m.rawTxCounter)
	assert.NotZero(t, m.postEFCounter)
	assert.NotZero(t, m.postTXCounter)
	assert.NotZero(t, m.merklePathCounter)
	assert.NotZero(t, m.findChainTipHeaderCounter)
	assert.NotZero(t, m.isValidRootForHeightCounter)
	assert.NotZero(t, m.currentHeightCounter)
	assert.NotZero(t, m.getScriptHashHistoryCounter)
	assert.NotZero(t, m.hashToHeaderCounter)
	assert.NotZero(t, m.chainHeaderByHeightCounter)
	assert.NotZero(t, m.getStatusForTxIDsCounter)
	assert.NotZero(t, m.getUtxoStatusCounter)
	assert.NotZero(t, m.isUtxoCounter)
	assert.NotZero(t, m.bsvExchangeRateCounter)
}

func callAllMethods(t testing.TB, service *services.WalletServices) []error {
	errs := make([]error, 0, 13)

	_, err := service.RawTx(t.Context(), mockTxID)
	errs = append(errs, err)

	beef, err := transaction.NewBeefFromTransaction(mockTx.TX())
	require.NoError(t, err)

	_, err = service.PostFromBEEF(t.Context(), beef, []string{mockTxID})
	errs = append(errs, err)
	_, err = service.MerklePath(t.Context(), mockTxID)
	errs = append(errs, err)

	_, err = service.FindChainTipHeader(t.Context())
	errs = append(errs, err)

	_, err = service.IsValidRootForHeight(t.Context(), &chainhash.Hash{}, 0)
	errs = append(errs, err)

	_, err = service.CurrentHeight(t.Context())
	errs = append(errs, err)

	_, err = service.GetScriptHashHistory(t.Context(), "scriptHash")
	errs = append(errs, err)

	_, err = service.HashToHeader(t.Context(), "hash")
	errs = append(errs, err)

	_, err = service.ChainHeaderByHeight(t.Context(), 0)
	errs = append(errs, err)

	_, err = service.GetStatusForTxIDs(t.Context(), []string{mockTxID})
	errs = append(errs, err)

	_, err = service.GetUtxoStatus(t.Context(), "scriptHash", &transaction.Outpoint{})
	errs = append(errs, err)

	_, err = service.IsUtxo(t.Context(), "scriptHash", &transaction.Outpoint{})
	errs = append(errs, err)

	_, err = service.BsvExchangeRate(t.Context())
	errs = append(errs, err)

	return errs
}
