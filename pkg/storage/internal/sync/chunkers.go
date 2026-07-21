package sync

func all(repo Repository) []Chunker {
	return []Chunker{
		newChunkerBaskets(repo),
		// ProvenTxs must sync (and register their writer-side IDs) before ProvenTxReqs, since a
		// ProvenTxReq references its ProvenTx by ID and each chunker drains fully before the next
		// one starts — reversing this order can leave a req's ProvenTx ID unresolved mid-sync.
		newChunkerProvenTxs(repo),
		newChunkerProvenTxReqs(repo),
		newChunkerUserTransactions(repo),
		newChunkerOutputs(repo),
		newChunkerLabels(repo),
		newChunkerLabelsMap(repo),
		newChunkerTags(repo),
		newChunkerTagsMap(repo),
	}
}
