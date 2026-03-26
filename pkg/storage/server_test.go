package storage_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestRPCCommunication(t *testing.T) {
	t.Run("Migrate", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		mockStorage.EXPECT().
			Migrate(gomock.Any(), fixtures.StorageName, fixtures.StorageIdentityKey).Times(0)

		// when:
		migrationVersion, err := client.Migrate(t.Context(), fixtures.StorageName, fixtures.StorageIdentityKey)

		// then:
		require.ErrorContains(t, err, "method not allowed to be called via RPC")
		assert.Empty(t, migrationVersion)
	})

	t.Run("MakeAvailable", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		storageResult := &wdk.TableSettings{
			StorageName:        fixtures.StorageName,
			StorageIdentityKey: fixtures.StorageIdentityKey,
			Chain:              defs.NetworkTestnet,
			MaxOutputScript:    1024,
		}

		mockStorage.EXPECT().
			MakeAvailable(gomock.Any()).
			Return(storageResult, nil)

		// when:
		response, err := client.MakeAvailable(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, storageResult, response)
	})

	t.Run("FindOrInsertUser", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		userIdentityKey := testusers.Alice.IdentityKey(t)

		storageResult := &wdk.FindOrInsertUserResponse{
			User: wdk.TableUser{
				IdentityKey: userIdentityKey,
			},
			IsNew: false,
		}

		// and:
		mockStorage.EXPECT().
			FindOrInsertUser(gomock.Any(), userIdentityKey).
			Return(storageResult, nil)

		// when:
		response, err := client.FindOrInsertUser(t.Context(), userIdentityKey)

		// then:
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, storageResult, response)
	})

	t.Run("Internalize", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		args := wdk.InternalizeActionArgs{
			Tx: tsgenerated.ParentTransactionAtomicBeef(t),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.WalletPaymentProtocol,
					PaymentRemittance: &wdk.WalletPayment{
						DerivationPrefix:  fixtures.DerivationPrefix,
						DerivationSuffix:  fixtures.DerivationSuffix,
						SenderIdentityKey: fixtures.AnyoneIdentityKey,
					},
				},
			},
			Labels: []primitives.StringUnder300{
				"label1", "label2",
			},
			Description:    "description",
			SeekPermission: nil,
		}

		storageResult := &wdk.InternalizeActionResult{
			Accepted: true,
			IsMerge:  false,
			TxID:     "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
			Satoshis: 99904,
		}

		// and:
		mockStorage.EXPECT().
			InternalizeAction(gomock.Any(), testusers.Alice.AuthID(), gomock.Eq(args)).
			Return(storageResult, nil)

		// when:
		result, err := client.InternalizeAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, storageResult, result)
	})

	t.Run("CreateAction", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		args := wdk.ValidCreateActionArgs{
			Description: "outputBRC29",
			Inputs:      nil,
			Outputs: []wdk.ValidCreateActionOutput{
				{
					LockingScript:      "76a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac",
					Satoshis:           1000,
					OutputDescription:  "outputBRC29",
					CustomInstructions: to.Ptr(`{"derivationPrefix":"Pr==","derivationSuffix":"Su==","type":"BRC29"}`),
				},
			},
			LockTime: 0,
			Version:  1,
			Labels:   []primitives.StringUnder300{"outputbrc29"},
			Options: wdk.ValidCreateActionOptions{
				AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
				SendWith:               nil,
				SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
				KnownTxids:             nil,
				NoSendChange:           nil,
				RandomizeOutputs:       false,
			},
			IsSendWith:                   false,
			IsDelayed:                    false,
			IsNoSend:                     false,
			IsNewTx:                      true,
			IsRemixChange:                false,
			IsSignAction:                 false,
			IncludeAllSourceTransactions: true,
		}

		storageResult := &wdk.StorageCreateActionResult{
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
		}

		// and:
		mockStorage.EXPECT().
			CreateAction(gomock.Any(), testusers.Alice.AuthID(), args).
			Return(storageResult, nil)

		// when:
		result, err := client.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, storageResult, result)
	})

	t.Run("ProcessAction", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		tx, err := transaction.NewTransactionFromHex("0100000001f65e47ff7a34ad0e84d70cf2100cdca1c07129859ad8365ce0008fadd5546775000000006b483045022100da4152b91408074c6008f234e226a5b9f2a0b041ab16e2bced3ee01ae33ade5e02207f2ba65b4c1dee2dff28dda07c5fb86b896b9dcaa25a3206803c6aa308ceccad412102c6a89106be8c0ac3beda8369213d0b6158054563cfc34d5daef1d6677bb066aaffffffff20e8030000000000001976a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac820c0000000000001976a91469e18abccfe542357d10b79b7bba36def766023788ac760c0000000000001976a914f3218db3a0674b1401ad6a65017ae327ac176b2588ac760c0000000000001976a914d7d1694337818f830600253e2efbf8e54aa5770288ac760c0000000000001976a914e450687ade72e67cfc0bdcc069b71dea0347fb4188ac760c0000000000001976a91459d9d1aa277f9972fffb0a89e938f10f3457fc2588ac760c0000000000001976a91494a3db90470b4ea4e8f3f5b435ef5703a79cb4eb88ac760c0000000000001976a914e6223cd0da6dfd8a1ed2b57edbd0e12e6a8f549988ac760c0000000000001976a914885e7bdd1b79da43d11953eef3389602478caf8188ac760c0000000000001976a9146a7109c7597d968bc01e496c4e4506d6ee192ece88ac760c0000000000001976a91420d4019a6b8ce3fd83bd1d56c8e33867afa27a6788ac760c0000000000001976a9142427cddab6b96eb8f74bcb897162575c04aea82588ac760c0000000000001976a91407cb2c039de852d845061c339c8d493d1a3fd7c088ac760c0000000000001976a9144af9c016a932c8b8c160e29701fc1c7a9dd8b99388ac760c0000000000001976a9142b95ea36f4a4fad0078abea544295d89e8c989aa88ac760c0000000000001976a914c0f133dc94fcc893677c55c8269071c7807ba41988ac760c0000000000001976a9147d431ea84cc215c964baba5273d0377a3d22fc5688ac760c0000000000001976a9149eda2678c5f4e36c44f1b24280e238e93aeadde188ac760c0000000000001976a9143886edfde546620d3445cba9410315a31b74ba2888ac760c0000000000001976a914f0cbdce1a943b2c197f7f270ee90a09f2b13484488ac760c0000000000001976a914b4bb7090ee8e4ce4524e2ba5040e16f962b8585d88ac760c0000000000001976a914e479d181acbe4e34794cc675235541124d4c807088ac760c0000000000001976a914c47f2bc86cae092f0566ee760e022a12375ab11588ac760c0000000000001976a914012d0269e7c0fff4769e0117178cb3973577ee9c88ac760c0000000000001976a914103fed827ba775a057d3f63b014c69b098ebadfa88ac760c0000000000001976a9143c97d8f6dd0b4fb133190a8a68fe8653ff5a044d88ac760c0000000000001976a9140a5ecc1c39fb2599f5922d83e7a93ccabd24a7fa88ac760c0000000000001976a914fac479fe17ebd028b242d1a6f0f3de17b02a2fac88ac760c0000000000001976a914ff3e2f7f73101facfee37b4c41325db8ef808da788ac760c0000000000001976a9147f8651a2f0f360206bf3bdaa75c876dd8e5e790b88ac760c0000000000001976a9143a9f3078af876700d8ac9e108d722dee44bae05d88ac760c0000000000001976a9147bf6ebec663a3801d11fec1d7db6abd7b3dfc30688ac00000000")
		require.NoError(t, err)

		args := wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  false,
			Reference:  to.Ptr("Y2NjY2NjY2NjY2Nj"),
			TxID:       to.Ptr(primitives.TXIDHexString(tx.TxID().String())),
			RawTx:      tx.Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}

		storageResult := &wdk.ProcessActionResult{
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
		}

		// and:
		mockStorage.EXPECT().
			ProcessAction(gomock.Any(), testusers.Alice.AuthID(), args).
			Return(storageResult, nil)

		// when:
		result, err := client.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, storageResult, result)
	})

	t.Run("InsertCertificate", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))

		mockStorage.EXPECT().
			InsertCertificateAuth(gomock.Any(), testusers.Alice.AuthID(), certToInsert).
			Return(uint(1), nil)

		// when:
		result, err := client.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)
		assert.EqualValues(t, 1, result)
	})

	t.Run("RelinquishCertificate", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		relinquishArgs := wdk.RelinquishCertificateArgs{
			Type:         "ZXhhbXBsZVR5cGUy",
			SerialNumber: fixtures.SerialNumber,
			Certifier:    fixtures.Certifier,
		}

		mockStorage.EXPECT().
			RelinquishCertificate(gomock.Any(), testusers.Alice.AuthID(), relinquishArgs).
			Return(nil)

		// when:
		err := client.RelinquishCertificate(t.Context(), testusers.Alice.AuthID(), relinquishArgs)

		// then:
		require.NoError(t, err)
	})

	t.Run("ListCertificates", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		listArgs := wdk.ListCertificatesArgs{}

		storageResult := &wdk.ListCertificatesResult{
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
		}

		mockStorage.EXPECT().
			ListCertificates(gomock.Any(), testusers.Alice.AuthID(), listArgs).
			Return(storageResult, nil)

		// when:
		response, err := client.ListCertificates(t.Context(), testusers.Alice.AuthID(), listArgs)

		// then:
		require.NoError(t, err)
		require.Equal(t, storageResult, response)
	})

	t.Run("ListOutputs", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// given:
		listArgs := wdk.ListOutputsArgs{
			Basket:              "",
			Limit:               10,
			Offset:              0,
			KnownTxids:          []string{"3105f51688f7081b6b1c364ec6455787e3b6765626f8a2ebf76084be8540453b"},
			IncludeTransactions: true,
		}

		expectedResult := &wdk.ListOutputsResult{
			TotalOutputs: 1,
			BEEF:         primitives.ExplicitByteArray{0x01, 0x02},
			Outputs: []*wdk.WalletOutput{
				{
					Satoshis:  1000,
					Spendable: true,
					Outpoint:  "3105f51688f7081b6b1c364ec6455787e3b6765626f8a2ebf76084be8540453b.0",
				},
			},
		}

		mockStorage.EXPECT().
			ListOutputs(gomock.Any(), testusers.Alice.AuthID(), listArgs).
			Return(expectedResult, nil)

		// when:
		actualResult, err := client.ListOutputs(t.Context(), testusers.Alice.AuthID(), listArgs)

		// then:
		require.NoError(t, err)
		require.NotNil(t, actualResult)
		assert.Equal(t, expectedResult, actualResult)
	})

	t.Run("ListActions", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// Given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		args := wdk.ListActionsArgs{
			Limit:  10,
			Offset: 0,
		}

		expectedResult := &wdk.ListActionsResult{
			TotalActions: 1,
			Actions: []wdk.WalletAction{
				{
					TxID:        "abcd1234",
					Satoshis:    1000,
					Status:      "completed",
					IsOutgoing:  false,
					Description: "Test transaction",
					Version:     1,
					LockTime:    0,
					Labels:      []string{"label1"},
				},
			},
		}

		mockStorage.EXPECT().
			ListActions(gomock.Any(), testusers.Alice.AuthID(), args).
			Return(expectedResult, nil)

		// When:
		actualResult, err := client.ListActions(t.Context(), testusers.Alice.AuthID(), args)

		// Then:
		require.NoError(t, err)
		require.NotNil(t, actualResult)
		assert.Equal(t, expectedResult, actualResult)
	})
}

func TestServerAuthentication(t *testing.T) {
	t.Run("reject FindOrInsertUser when identity key doesn't match", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		mockStorage.EXPECT().FindOrInsertUser(gomock.Any(), gomock.Any()).Times(0)

		// when: alice tries to insert bob
		response, err := client.FindOrInsertUser(t.Context(), testusers.Bob.IdentityKey(t))

		// then:
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "identityKey does not match authentication")
		}
		assert.Nil(t, response)
	})

	t.Run("reject method with authid when identity key doesn't match", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		mockStorage.EXPECT().InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		// when: alice tries to internalize transaction for bob
		response, err := client.InternalizeAction(t.Context(), testusers.Bob.AuthID(), wdk.InternalizeActionArgs{})

		// then:
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "identityKey does not match authentication")
		}
		assert.Nil(t, response)
	})

	t.Run("use the correct user id when provided user id doesn't match the one for identity key", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		userIdentityKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			FindOrInsertUser(gomock.Any(), userIdentityKey).
			Return(
				&wdk.FindOrInsertUserResponse{
					User: wdk.TableUser{
						IdentityKey: userIdentityKey,
						UserID:      testusers.Alice.ID,
					}, IsNew: false,
				},
				nil,
			).MinTimes(0)

		mockStorage.EXPECT().
			InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(_ any, authID wdk.AuthID, _ any) {
				if assert.NotNil(t, authID.UserID) {
					assert.Equal(t, testusers.Alice.ID, *authID.UserID)
					assert.NotEqual(t, testusers.Bob.ID, *authID.UserID)
				}
			})

		// when: alice internalizes action with not her user id
		authID := testusers.Alice.AuthID()

		authID.UserID = to.Ptr(testusers.Bob.ID)

		_, err := client.InternalizeAction(t.Context(), authID, wdk.InternalizeActionArgs{})

		// then: important assertions are done in mock storage
		require.NoError(t, err)
	})
}

func TestServerRequestMonetization(t *testing.T) {
	t.Run("require payment of default amount for request", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage, func(opt *storage.ServerOptions) {
			opt.Monetize = true
		})
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		userIdentityKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			FindOrInsertUser(gomock.Any(), userIdentityKey).
			Return(
				&wdk.FindOrInsertUserResponse{
					User: wdk.TableUser{
						IdentityKey: userIdentityKey,
						UserID:      testusers.Alice.ID,
					}, IsNew: false,
				},
				nil,
			).MinTimes(0)

		mockStorage.EXPECT().
			InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, authID wdk.AuthID, _ any) {
				paymentInfo, err := middleware.ShouldGetPaymentInfo(ctx)
				if assert.NoError(t, err) && assert.NotNil(t, paymentInfo) {
					assert.Equal(t, 100, paymentInfo.SatoshisPaid, "should pay default amount")
					assert.True(t, paymentInfo.Accepted)
				}
			})

		// when:
		authID := testusers.Alice.AuthID()

		_, err := client.InternalizeAction(t.Context(), authID, wdk.InternalizeActionArgs{})

		// then:
		require.NoError(t, err)
	})

	t.Run("require payment of custom amount for request", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		customAmount := 1000

		// and:
		mockStorage := given.MockProvider()

		// and server:
		cleanupSrv := given.StartedRPCServerFor(mockStorage, func(opt *storage.ServerOptions) {
			opt.Monetize = true
			opt.CalculateRequestPrice = func(r *http.Request) (int, error) {
				return customAmount, nil
			}
		})
		defer cleanupSrv()

		// and client:
		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// and:
		userIdentityKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			FindOrInsertUser(gomock.Any(), userIdentityKey).
			Return(
				&wdk.FindOrInsertUserResponse{
					User: wdk.TableUser{
						IdentityKey: userIdentityKey,
						UserID:      testusers.Alice.ID,
					}, IsNew: false,
				},
				nil,
			).MinTimes(0)

		mockStorage.EXPECT().
			InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, authID wdk.AuthID, _ any) {
				paymentInfo, err := middleware.ShouldGetPaymentInfo(ctx)
				if assert.NoError(t, err) && assert.NotNil(t, paymentInfo) {
					assert.Equal(t, customAmount, paymentInfo.SatoshisPaid, "should pay custom amount")
					assert.True(t, paymentInfo.Accepted)
				}
			})

		// when:
		authID := testusers.Alice.AuthID()

		_, err := client.InternalizeAction(t.Context(), authID, wdk.InternalizeActionArgs{})

		// then:
		require.NoError(t, err)
	})
}
