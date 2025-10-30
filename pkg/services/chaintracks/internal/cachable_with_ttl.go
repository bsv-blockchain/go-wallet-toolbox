package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
)

type CachableWithTTL[T any] struct {
	value   *T
	lastSet *time.Time

	setter func(context.Context) (T, error)
	ttl    time.Duration
	locker sync.RWMutex
}

func NewCachableWithTTL[T any](ttl time.Duration, setter func(context.Context) (T, error)) *CachableWithTTL[T] {
	return &CachableWithTTL[T]{
		ttl: ttl,
		setter: setter,
	}
}

func (c *CachableWithTTL[T]) Get(ctx context.Context) (T, error) {
	cachedValue := c.readOnValid()
	if cachedValue != nil {
		return *cachedValue, nil
	}

	c.locker.Lock()
	defer c.locker.Unlock()

	// Double check after acquiring write lock
	// Other goroutine might have set the value in the meantime
	if c.isValid() {
		return *c.value, nil
	}

	// The lock for all readers for the time of running the setter is intentional
	// All reads must wait for it and then get the new value from cache
	newVal, err := c.setter(ctx)
	if err != nil {
		return to.EmptyValue[T](), fmt.Errorf("failed to set cache value: %w", err)
	}

	c.value = to.Ptr(newVal)
	c.lastSet = to.Ptr(time.Now())
	return *c.value, nil
}

func (c *CachableWithTTL[T]) readOnValid() *T {
	c.locker.RLock()
	defer c.locker.RUnlock()
	if c.isValid() {
		return c.value
	}
	return nil
}

func (c *CachableWithTTL[T]) isValid() bool {
	if c.value == nil || c.lastSet == nil {
		return false
	}
	return time.Since(*c.lastSet) < c.ttl
}
