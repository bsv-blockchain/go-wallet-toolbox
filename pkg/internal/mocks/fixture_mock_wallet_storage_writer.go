package mocks

import (
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"go.uber.org/mock/gomock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

var DefaultTimestamp time.Time = time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)

type StorageProviderResponses struct {
	Migrate               StorageProviderMethodResponse[string]
	MakeAvailable         StorageProviderMethodResponse[*wdk.TableSettings]
	FindOrInsertUser      StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse]
	InternalizeAction     StorageProviderMethodResponse[*wdk.InternalizeActionResult]
	CreateAction          StorageProviderMethodResponse[*wdk.StorageCreateActionResult]
	ProcessAction         StorageProviderMethodResponse[*wdk.ProcessActionResult]
	InsertCertificateAuth StorageProviderMethodResponse[uint]
	RelinquishCertificate StorageProviderMethodOnlyErrorResponse
	RelinquishOutput      StorageProviderMethodOnlyErrorResponse
	ListCertificates      StorageProviderMethodResponse[*wdk.ListCertificatesResult]
	ListOutputs           StorageProviderMethodResponse[*wdk.ListOutputsResult]
}

type StorageProviderMethodOnlyErrorResponse struct {
	Error    error
	maxCalls *int
}

func (r *StorageProviderMethodOnlyErrorResponse) times(max *int) {
	r.maxCalls = max
}

func (r *StorageProviderMethodOnlyErrorResponse) result() error {
	return r.Error
}

func (r *StorageProviderMethodOnlyErrorResponse) limitCallTimes(call *gomock.Call) {
	if r.maxCalls == nil {
		return
	}
	call.MaxTimes(*r.maxCalls)
}

type StorageProviderMethodResponse[T any] struct {
	Success  T
	Error    error
	maxCalls *int
}

func (r *StorageProviderMethodResponse[T]) success() T {
	if r.Error != nil {
		return to.ZeroValue[T]()
	}
	return r.Success
}

func (r *StorageProviderMethodResponse[T]) error() error {
	return r.Error
}

func (r *StorageProviderMethodResponse[T]) result() (T, error) {
	return r.success(), r.error()
}

func (r *StorageProviderMethodResponse[T]) times(max *int) {
	r.maxCalls = max
}

func (r *StorageProviderMethodResponse[T]) limitCallTimes(call *gomock.Call) {
	if r.maxCalls == nil {
		return
	}
	call.MaxTimes(*r.maxCalls)
}

func Once[T interface{ times(max *int) }]() func(T) {
	return func(response T) {
		response.times(to.Ptr(1))
	}
}

func WithMakeAvailableResponse(override func(response *StorageProviderMethodResponse[*wdk.TableSettings])) func(*StorageProviderResponses) {
	if override == nil {
		return func(responses *StorageProviderResponses) {}
	}
	return func(responses *StorageProviderResponses) {
		override(&responses.MakeAvailable)
	}
}

func WithFindOrInsertUserResponse(override func(response *StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse])) func(*StorageProviderResponses) {
	if override == nil {
		return func(responses *StorageProviderResponses) {}
	}
	return func(responses *StorageProviderResponses) {
		override(&responses.FindOrInsertUser)
	}
}

func ExpectNoInteraction() func(*StorageProviderResponses) {
	return func(responses *StorageProviderResponses) {
		zero := to.Ptr(0)
		responses.Migrate.times(zero)
		responses.MakeAvailable.times(zero)
		responses.FindOrInsertUser.times(zero)
		responses.InternalizeAction.times(zero)
		responses.CreateAction.times(zero)
		responses.ProcessAction.times(zero)
		responses.InsertCertificateAuth.times(zero)
		responses.RelinquishCertificate.times(zero)
		responses.RelinquishOutput.times(zero)
		responses.ListCertificates.times(zero)
		responses.ListOutputs.times(zero)
	}
}

func SetupMockStorageProvider(t testing.TB, provider *MockWalletStorageProvider, opts ...func(*StorageProviderResponses)) {
	responses := to.OptionsWithDefault(DefaultResponses(t), opts...)

	responses.Migrate.limitCallTimes(provider.EXPECT().Migrate(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.Migrate.result()))
	responses.MakeAvailable.limitCallTimes(provider.EXPECT().MakeAvailable(gomock.Any()).AnyTimes().Return(responses.MakeAvailable.result()))
	responses.FindOrInsertUser.limitCallTimes(provider.EXPECT().FindOrInsertUser(gomock.Any(), gomock.Any()).AnyTimes().Return(responses.FindOrInsertUser.result()))
	responses.InternalizeAction.limitCallTimes(provider.EXPECT().InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.InternalizeAction.result()))
	responses.CreateAction.limitCallTimes(provider.EXPECT().CreateAction(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.CreateAction.result()))
	responses.ProcessAction.limitCallTimes(provider.EXPECT().ProcessAction(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.ProcessAction.result()))
	responses.InsertCertificateAuth.limitCallTimes(provider.EXPECT().InsertCertificateAuth(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.InsertCertificateAuth.result()))
	responses.RelinquishCertificate.limitCallTimes(provider.EXPECT().RelinquishCertificate(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.RelinquishCertificate.result()))
	responses.RelinquishOutput.limitCallTimes(provider.EXPECT().RelinquishOutput(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.RelinquishOutput.result()))
	responses.ListCertificates.limitCallTimes(provider.EXPECT().ListCertificates(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.ListCertificates.result()))
	responses.ListOutputs.limitCallTimes(provider.EXPECT().ListOutputs(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(responses.ListOutputs.result()))
}

func DefaultResponses(t testing.TB) StorageProviderResponses {
	return StorageProviderResponses{
		Migrate: StorageProviderMethodResponse[string]{
			Success: "current-migration-version",
		},
		MakeAvailable: StorageProviderMethodResponse[*wdk.TableSettings]{
			Success: &wdk.TableSettings{
				StorageName:        fixtures.StorageName,
				StorageIdentityKey: fixtures.StorageIdentityKey,
				Chain:              defs.NetworkTestnet,
				MaxOutputScript:    1024,
			},
		},
		FindOrInsertUser: StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse]{
			Success: &wdk.FindOrInsertUserResponse{
				User: wdk.TableUser{
					CreatedAt:     DefaultTimestamp,
					UpdatedAt:     DefaultTimestamp,
					IdentityKey:   testusers.Alice.IdentityKey(t),
					UserID:        testusers.Alice.ID,
					ActiveStorage: fixtures.StorageIdentityKey,
				},
				IsNew: true,
			},
		},
		InternalizeAction: StorageProviderMethodResponse[*wdk.InternalizeActionResult]{
			Success: &wdk.InternalizeActionResult{
				Accepted: true,
				IsMerge:  false,
				TxID:     "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
				Satoshis: 99904,
			},
		},
		CreateAction: StorageProviderMethodResponse[*wdk.StorageCreateActionResult]{
			Success: &wdk.StorageCreateActionResult{
				InputBeef: primitives.ExplicitByteArray{0x2, 0x0, 0xbe, 0xef, 0x0, 0x0},
				Inputs: []*wdk.StorageCreateTransactionSdkInput{
					{
						Vin:                   0,
						SourceTxID:            "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
						SourceVout:            0x0,
						SourceSatoshis:        1101,
						SourceLockingScript:   "76a914a7d6e4270f5c90cc9e272586a6a5099663572d5988ac",
						SourceTransaction:     nil,
						UnlockingScriptLength: to.Ptr[primitives.PositiveInteger](txutils.P2PKHUnlockingScriptLength),
						ProvidedBy:            "storage",
						Type:                  "P2PKH",
						SpendingDescription:   nil,
						DerivationPrefix:      to.Ptr("Pr=="),
						DerivationSuffix:      to.Ptr("Su=="),
						SenderIdentityKey:     nil,
					},
				},
				Outputs: []*wdk.StorageCreateTransactionSdkOutput{
					{
						ValidCreateActionOutput: wdk.ValidCreateActionOutput{
							LockingScript:      "76a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac",
							Satoshis:           1000,
							OutputDescription:  "outputBRC29",
							Basket:             nil,
							CustomInstructions: to.Ptr("{\"derivationPrefix\":\"Pr==\",\"derivationSuffix\":\"Su==\",\"type\":\"BRC29\"}"),
							Tags:               nil,
						},
						Vout:             0,
						ProvidedBy:       "you",
						Purpose:          "",
						DerivationSuffix: nil,
					},
					{
						ValidCreateActionOutput: wdk.ValidCreateActionOutput{
							LockingScript:      "",
							Satoshis:           100,
							OutputDescription:  "",
							Basket:             to.Ptr(primitives.StringUnder300("default")),
							CustomInstructions: nil,
							Tags:               nil,
						},
						Vout:             0x1,
						ProvidedBy:       "storage",
						Purpose:          "change",
						DerivationSuffix: to.Ptr("ZGRkZGRkZGRkZGRkZGRkZA=="),
					},
				},
				NoSendChangeOutputVouts: nil,
				DerivationPrefix:        "YmJiYmJiYmJiYmJiYmJiYg==",
				Version:                 1,
				LockTime:                0,
				Reference:               "Y2NjY2NjY2NjY2Nj",
			},
		},
		ProcessAction: StorageProviderMethodResponse[*wdk.ProcessActionResult]{
			Success: &wdk.ProcessActionResult{
				SendWithResults: []wdk.SendWithResult{
					{
						TxID:   "3105f51688f7081b6b1c364ec6455787e3b6765626f8a2ebf76084be8540453b",
						Status: "unproven",
					},
				},
				NotDelayedResults: []wdk.ReviewActionResult{
					{
						TxID:          "3105f51688f7081b6b1c364ec6455787e3b6765626f8a2ebf76084be8540453b",
						Status:        "success",
						CompetingTxs:  nil,
						CompetingBeef: nil,
					},
				},
				Log: nil,
			},
		},
		InsertCertificateAuth: StorageProviderMethodResponse[uint]{
			Success: uint(1),
		},
		RelinquishOutput: StorageProviderMethodOnlyErrorResponse{
			Error: nil,
		},
		ListCertificates: StorageProviderMethodResponse[*wdk.ListCertificatesResult]{
			Success: &wdk.ListCertificatesResult{
				TotalCertificates: primitives.PositiveInteger(1),
				Certificates: []*wdk.CertificateResult{{
					Verifier: "",
					WalletCertificate: wdk.WalletCertificate{
						Type:               fixtures.TypeField,
						Subject:            fixtures.SubjectPubKey,
						SerialNumber:       fixtures.SerialNumber,
						Certifier:          fixtures.Certifier,
						RevocationOutpoint: fixtures.RevocationOutpoint,
						Signature:          fixtures.Signature,
						Fields: map[primitives.StringUnder50Bytes]string{
							"exampleField": "exampleValue",
						},
					},
					Keyring: map[primitives.StringUnder50Bytes]primitives.Base64String{
						"exampleField": "exampleValue",
					},
				}},
			},
		},
		ListOutputs: StorageProviderMethodResponse[*wdk.ListOutputsResult]{
			Success: &wdk.ListOutputsResult{
				TotalOutputs: 1,
				BEEF:         primitives.ExplicitByteArray{1, 2},
				Outputs: []*wdk.WalletOutput{
					{
						Satoshis:  1000,
						Spendable: true,
						Outpoint:  "3105f51688f7081b6b1c364ec6455787e3b6765626f8a2ebf76084be8540453b.0",
					},
				},
			},
		},
	}
}
