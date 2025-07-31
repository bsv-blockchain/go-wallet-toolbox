package testabilities

import "fmt"

func BlockHeaderJSON(hash string, height int, version int, merkleRoot string, time int64, nonce int, bits string, previous string) string {
	return fmt.Sprintf(`{
		"hash": "%s",
		"height": %d,
		"version": %d,
		"merkleroot": "%s",
		"time": %d,
		"nonce": %d,
		"bits": "%s",
		"previousblockhash": "%s"
	}`, hash, height, version, merkleRoot, time, nonce, bits, previous)
}

func InvalidBitsBlockHeaderJSON(hash string, merkleRoot string) string {
	return BlockHeaderJSON(
		hash,
		100000,
		1,
		merkleRoot,
		1600000000,
		42,
		"zzz_invalid_hex",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	)
}

func IncompleteBlockHeaderJSON(hash string, merkleRoot string) string {
	return fmt.Sprintf(`{
		"hash": "%s",
		"height": %d,
		"merkleroot": "%s",
		"time": %d,
		"nonce": %d
	}`, hash, 100000, merkleRoot, 1600000000, 42)
}

func ValidBlockHeaderJSON(hash string, height int, version int, merkleRoot string, time uint32, nonce int, bits string, prevHash string) string {
	return fmt.Sprintf(`{
		"hash": "%s",
		"height": %d,
		"version": %d,
		"merkleroot": "%s",
		"time": %d,
		"nonce": %d,
		"bits": "%s",
		"previousblockhash": "%s"
	}`, hash, height, version, merkleRoot, time, nonce, bits, prevHash)
}

func DefaultBlockHeaderJSON() string {
	return ValidBlockHeaderJSON(
		TestTargetHash,
		TestBlockHeight,
		536870912,
		TestMerkleRootHex,
		1712345678,
		123456789,
		"1803a30c",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
}

// IncompleteBlockHeaderRaw returns a raw block header with missing fields.
func IncompleteBlockHeaderRaw() string {
	return "00000020aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
