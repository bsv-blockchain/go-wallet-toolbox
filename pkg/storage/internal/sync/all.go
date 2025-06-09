package sync

func all(repo Repository) []Chunker {
	return []Chunker{
		newChunkerBaskets(repo),
	}
}
