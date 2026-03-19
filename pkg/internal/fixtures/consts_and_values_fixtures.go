package fixtures

import (
	"encoding/base64"
	"fmt"

	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testhelper"
)

const (
	StorageServerPrivKey       = "8143f5ed6c5b41c3d084d39d49e161d8dde4b50b0685a4e4ac23959d3b8a319b"
	StorageIdentityKey         = "028f2daab7808b79368d99eef1ebc2d35cdafe3932cafe3d83cf17837af034ec29" // that matches StorageServerPrivKey
	StorageName                = "test-storage"
	SecondStorageServerPrivKey = "57fe31d0d0ae563ec47468106b5ce6fa50b2b38da07254fd1e109a15c43341ac"
	SecondStorageIdentityKey   = "03ee699bfe59c6aa13093360997fa8ad31d43cb798fa3f334aabcb53c1ac396601" // that matches SecondStorageServerPrivKey
	SecondStorageName          = "test-storage-2"
	StorageHandlerName         = "storage_server"
	UserIdentityKeyHex         = "03f17660f611ce531402a2ce1e070380b6fde57aca211d707bfab27bce42d86beb"
	DerivationPrefix           = "Pg=="
	DerivationSuffix           = "Sg=="
	CustomBasket               = "custom-basket"
	ExpectedValueToInternalize = 999
	AnyoneIdentityKey          = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798" // generated in TS by: new PrivateKey(1).toPublicKey().toString()
	Reference                  = "0LFT4CuWAMEgEa7I"
	MockOutpoint               = "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6.1" // example outpoint for testing purposes
	CreateActionTestLabel      = "test_label=true"
	CreateActionTestTag        = "test_tag=true"

	CreateActionTestCustomInstructions = `{"derivationPrefix":"bPRI9FYwsIo=","derivationSuffix":"FdjLdpnLnJM=","type":"BRC29"}`
	Limit                              = 100
	Offset                             = 0
)

var (
	DerivationPrefixBytes = testhelper.BytesFromBase64(DerivationPrefix)
	DerivationSuffixBytes = testhelper.BytesFromBase64(DerivationSuffix)
	UserIdentityKey       = testhelper.IdentityKeyFromHex(UserIdentityKeyHex)
	WalletPagingLimit     = to.Ptr[uint32](Limit)
	WalletPagingOffset    = to.Ptr[uint32](Offset)
	WalletLockTime        = to.Ptr[uint32](0)
	WalletTxVersion       = to.Ptr[uint32](1)
)

func FaucetTag(index int) string {
	return fmt.Sprintf("faucet-%d", index)
}

func FaucetReference(txID string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("faucet-reference-for-txid-%s", txID)))
}
