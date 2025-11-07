package chaintracks

var dirtyHashes = map[string]struct{}{
	// This is the first header of the invalid SegWit chain.
	"00000000000000000019f112ec0a9982926f1258cdcc558dd7c3b7e5dc7fa148": {},

	// This is the first header of the invalid ABC chain.
	"0000000000000000004626ff6e3b936941d341c5932ece4357eeccac44e6d56c": {},
}

// IsDirtyHash returns true if the provided hash is marked as dirty due to known invalid blockchain headers.
func IsDirtyHash(hash string) bool {
	_, exists := dirtyHashes[hash]
	return exists
}
