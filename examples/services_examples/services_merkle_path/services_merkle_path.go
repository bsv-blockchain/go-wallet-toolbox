package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

/*
Example output for a successful Merkle Path retrieval:
{
    "hash": "000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641",
    "confirmations": 1,
    "size": 3072008,
    "height": 903321,
    "version": 771751936,
    "versionHex": "2e000000",
    "merkleroot": "559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd",
    "txcount": 2081,
    "time": 1751198131,
    "mediantime": 1751193155,
    "nonce": 1179404358,
    "bits": "181606c9",
    "difficulty": 49916901237.38432,
    "chainwork": "00000000000000000000000000000000000000000166fb665858a1d5a2a1d5ab",
    "previousblockhash": "00000000000000001523ba5d75d66b2f8314c2d7a2476d8a19c8e84ab51c53bd",
    "nextblockhash": "",
    "coinbaseTx": {
        "hex": "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170399c80d2f43555656452f0150cbfa27d51703e1a32500ffffffff01f3d4a112000000001976a914d648686cf603c11850f39600e37312738accca8f88ac00000000",
        "txid": "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6",
        "hash": "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6",
        "size": 108,
        "version": 1,
        "locktime": 0,
        "vin": [
            {
                "n": 0,
                "coinbase": "0399c80d2f43555656452f0150cbfa27d51703e1a32500",
                "sequence": 4294967295,
                "minerInfo": {
                    "name": "CUVVE",
                    "type": "tag"
                }
            }
        ],
        "vout": [
            {
                "value": 3.12595699,
                "n": 0,
                "scriptPubKey": {
                    "asm": "OP_DUP OP_HASH160 d648686cf603c11850f39600e37312738accca8f OP_EQUALVERIFY OP_CHECKSIG",
                    "hex": "76a914d648686cf603c11850f39600e37312738accca8f88ac",
                    "reqSigs": 1,
                    "type": "pubkeyhash",
                    "addresses": [
                        "1LY2M3RCkEVKo82ym1SQ1iZGQhM5Lf5Pkf"
                    ],
                    "isTruncated": false
                },
                "scripthash": "020a5314df44ccfa5b8e5c5c5b354f397d2590832c40e032099f442b12fca370"
            }
        ],
        "blockhash": "000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641",
        "confirmations": 1,
        "time": 1751198131,
        "blocktime": 1751198131,
        "vincount": 1,
        "voutcount": 1,
        "voutvalue": 3.12595699
    },
    "totalFees": 0.0009569900000001574
}
*/

// processes services one by one until a successful result is obtained.
// The context and argument is passed to each service.
// Returns the first successful result or an error if all services fail.
// https://whatsonchain.com/block-height/903321?tab=json <-- Example of a block with Merkle Path
func main() {
	txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
	network := defs.NetworkMainnet

	serviceCfg := defs.DefaultServicesConfig(network)
	walletServices := services.New(slog.Default(), serviceCfg)

	result, err := walletServices.MerklePath(context.Background(), txID)
	if err != nil {
		panic(fmt.Errorf("failed to get MerklePath: %w", err))
	}

    show.MerklePathOutput(result, txID)
}

// Output:
// Merkle Path Metadata
// Field                 Value
// --------------------  ----------------------------------------------------------------
// Service               WhatsOnChain
// Block Hash            000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641
// Block Height          903321
// Merkle Root           559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd
// Computed Merkle Root  559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd
//
//
// Block Height: 903321 | Merkle Path for txID: 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6
// Level  Offset  Hash                                                              Is TxID
// -----  ------  ----------------------------------------------------------------  -------
// 0      0       9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6  yes
// 0      1       7614658ca0007fa36b4634a53ae3d4be5207414cccd2a418578b77df5ecce63b
// 1      1       1580364a629685228cb2527893da2553e93a0c8963d9993f76daf1a0d9becd36
// 2      1       f45a57b6c15a3ca2aa849fa85e224c75a9d9fcc3dffb783ec6445b872079d00f
// 3      1       a18f3c6fc6fd079a7a8a89a71ad134138418e2e1e8d42654eb7d4b788b47d800
// 4      1       44f1abc430ea7717f86ca084fd4a5cb20d71d9cb66e2395ec88b5d7bc58f441f
// 5      1       e8298fc5360ecfe64f22d2442097afcc6307b02d8b718d5588c8b2b07111407b
// 6      1       e27a8ad3d36d00ad37de836dde518fcfcba6c3067f6a5c227a37cddac877fec0
// 7      1       56b45af75b2f3d53f80baa93b7ec249b734c5655092805c0fe1d8933d36d517c
// 8      1       4cf9c5fffb8ee4f2d6c68786059bc54a980f050f99da9f627e21c82f2f1787c6
// 9      1       2d321206df2b0faea962902329fdd0a519e1d154925714bd284dc80c97b32cbd
// 10     1       3a27e54bf59f2612512519ce7d6315da551e4572d948fc8c9c5d0058ccfca608
// 11     1       53bb438fa84b1d17289d5bd5ce696350dc5a3887ab4011ea28dea8eecf1b137e