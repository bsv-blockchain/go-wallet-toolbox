package storage

// GroupCommitStats exposes the group committer's counters to the tests: committed batch
// transactions and the units of work they carried. Both are 0 when group commit is off.
func GroupCommitStats(p *Provider) (batches, units uint64) {
	if p.groupCommit == nil {
		return 0, 0
	}
	return p.groupCommit.Stats()
}
