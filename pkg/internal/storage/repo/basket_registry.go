package repo

import (
	"sync"
	"time"
)

// basketRegistryTTL bounds how long a "this basket exists" fact is trusted. It only matters when a
// basket row is removed behind this process's back (another instance, manual SQL); removals made
// through this process call forget explicitly.
const basketRegistryTTL = 30 * time.Second

type basketKey struct {
	userID int
	name   string
}

// basketRegistry remembers output baskets known to exist as COMMITTED rows, so the per-action
// "INSERT ... ON CONFLICT DO NOTHING" can be skipped. It is owned by one repository set (one
// database), never shared between databases, and is safe for concurrent use.
//
// A basket is recorded only when the guarded INSERT affected no row: the row was already there and,
// because the engines make a conflicting INSERT wait for an in-flight inserter to commit or roll
// back, it was committed. A basket this very transaction inserted is never recorded, so a rollback
// cannot leave a phantom entry (see TestCachedBasketMaker_*).
type basketRegistry struct {
	mu      sync.RWMutex
	entries map[basketKey]time.Time // key -> expiry
	// inserted records, per basket, the connection (transaction) that inserted the row most
	// recently, so a second call in that same transaction does not mistake its own uncommitted
	// row for a committed one. Entries older than the TTL are dropped.
	inserted map[basketKey]insertedBy
	now      func() time.Time
	ttl      time.Duration
}

func newBasketRegistry() *basketRegistry {
	return &basketRegistry{
		entries:  make(map[basketKey]time.Time),
		inserted: make(map[basketKey]insertedBy),
		now:      time.Now,
		ttl:      basketRegistryTTL,
	}
}

type insertedBy struct {
	conn any
	at   time.Time
}

// noteInserted records that the transaction behind conn inserted the basket row.
func (r *basketRegistry) noteInserted(userID int, name string, conn any) {
	now := r.now()
	r.mu.Lock()
	for k, v := range r.inserted {
		if now.Sub(v.at) > r.ttl {
			delete(r.inserted, k)
		}
	}
	r.inserted[basketKey{userID, name}] = insertedBy{conn: conn, at: now}
	r.mu.Unlock()
}

// rememberCommitted records the basket as existing, unless the row may be the uncommitted insert
// of the transaction behind conn itself.
func (r *basketRegistry) rememberCommitted(userID int, name string, conn any) {
	key := basketKey{userID, name}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ins, ok := r.inserted[key]; ok && ins.conn == conn && r.now().Sub(ins.at) <= r.ttl {
		return
	}
	r.entries[key] = r.now().Add(r.ttl)
}

func (r *basketRegistry) known(userID int, name string) bool {
	r.mu.RLock()
	expiry, ok := r.entries[basketKey{userID, name}]
	r.mu.RUnlock()

	return ok && r.now().Before(expiry)
}

// forget drops one basket; call it whenever a basket row is deleted.
func (r *basketRegistry) forget(userID int, name string) {
	r.mu.Lock()
	delete(r.entries, basketKey{userID, name})
	delete(r.inserted, basketKey{userID, name})
	r.mu.Unlock()
}

// forgetUser drops every basket of a user.
func (r *basketRegistry) forgetUser(userID int) {
	r.mu.Lock()
	for k := range r.entries {
		if k.userID == userID {
			delete(r.entries, k)
		}
	}
	for k := range r.inserted {
		if k.userID == userID {
			delete(r.inserted, k)
		}
	}
	r.mu.Unlock()
}
