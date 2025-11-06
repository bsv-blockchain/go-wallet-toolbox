package models

type InsertHeaderResult struct {
	// true only if the new header was inserted
	Added bool
	// true only if the header was not inserted because a matching hash already exists in the database
	Dupe bool
	// true only if the new header became the active chain tip
	IsActiveTip bool
	// zero if the insertion of the new header did not cause a reorg.
	ReorgDepth int
	// If `added` is true, this header was the active chain tip before the insert. It may or may not still be the active chain tip after the insert.
	PriorTip *LiveBlockHeader
	// If a reorg has occurred, these headers where active and are now deactivated.
	DeactivatedHeaders []LiveBlockHeader
	// header's previousHash was not found in database
	NoPrev bool
	// header matching previousHash does not have height - 1
	BadPrev bool
	// an active ancestor was not found in live storage or prev header.
	NoActiveAncestor bool
	// a current chain tip was not found in live storage or prev header.
	NoTip bool
}
