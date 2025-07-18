package sync

func all(repo Repository) []Chunker {
	return []Chunker{
		newChunkerBaskets(repo),
		newChunkerKnownTxs(repo),
		newChunkerUserTransactions(repo),
		newChunkerOutputs(repo),
		newChunkerLabels(repo),
		newChunkerLabelsMap(repo),
		newChunkerTags(repo),
		newChunkerTagsMap(repo),
	}
}
