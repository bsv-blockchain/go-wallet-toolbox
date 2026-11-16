package hsm

import (
	"container/list"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

// pubCache is an LRU of derived public keys. Entries are public points, not
// scalars. A non-positive max disables the cache.
type pubCache struct {
	max int
	ll  *list.List
	m   map[pubCacheKey]*list.Element
}

type pubCacheKey struct {
	level   int
	proto   string
	keyID   string
	peer    string
	forSelf bool
}

type pubCacheEntry struct {
	key pubCacheKey
	pub *ec.PublicKey
}

func newPubCache(max int) *pubCache {
	if max < 0 {
		max = 0
	}
	return &pubCache{
		max: max,
		ll:  list.New(),
		m:   make(map[pubCacheKey]*list.Element),
	}
}

func (c *pubCache) get(key pubCacheKey) (*ec.PublicKey, bool) {
	if c == nil || c.max == 0 {
		return nil, false
	}
	el, ok := c.m[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*pubCacheEntry).pub, true
}

func (c *pubCache) put(key pubCacheKey, pub *ec.PublicKey) {
	if c == nil || c.max == 0 || pub == nil {
		return
	}
	if el, ok := c.m[key]; ok {
		el.Value.(*pubCacheEntry).pub = pub
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&pubCacheEntry{key: key, pub: pub})
	c.m[key] = el
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		if back == nil {
			return
		}
		c.ll.Remove(back)
		delete(c.m, back.Value.(*pubCacheEntry).key)
	}
}
