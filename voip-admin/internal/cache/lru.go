package cache

import (
	"container/list"
	"sync"
	"time"
)

// entry represents a cache entry with expiration
type entry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

// LRUCache is a thread-safe LRU cache with TTL support
type LRUCache struct {
	capacity int
	items    map[string]*list.Element
	evictList *list.List
	mu       sync.RWMutex
	hits     uint64
	misses   uint64
}

// New creates a new LRU cache with specified capacity
func New(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}

	return &LRUCache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}

	entry := elem.Value.(*entry)

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		c.misses++
		return nil, false
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(elem)
	c.hits++

	return entry.value, true
}

// Set adds or updates a value in the cache with TTL
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(ttl)

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry
		c.evictList.MoveToFront(elem)
		entry := elem.Value.(*entry)
		entry.value = value
		entry.expiresAt = expiresAt
		return
	}

	// Add new entry
	entry := &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}

	elem := c.evictList.PushFront(entry)
	c.items[key] = elem

	// Evict oldest if capacity exceeded
	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.hits = 0
	c.misses = 0
}

// Len returns the number of items in the cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.evictList.Len()
}

// Stats returns cache statistics
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:     c.hits,
		Misses:   c.misses,
		HitRate:  hitRate,
		Size:     c.evictList.Len(),
		Capacity: c.capacity,
	}
}

// removeOldest removes the oldest item from the cache
func (c *LRUCache) removeOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes a specific element from the cache
func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*entry)
	delete(c.items, entry.key)
}

// Cleanup removes expired entries (should be called periodically)
func (c *LRUCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for elem := c.evictList.Back(); elem != nil; {
		entry := elem.Value.(*entry)
		prev := elem.Prev()

		if now.After(entry.expiresAt) {
			c.removeElement(elem)
			removed++
		}

		elem = prev
	}

	return removed
}

// CacheStats represents cache statistics
type CacheStats struct {
	Hits     uint64  `json:"hits"`
	Misses   uint64  `json:"misses"`
	HitRate  float64 `json:"hit_rate"`
	Size     int     `json:"size"`
	Capacity int     `json:"capacity"`
}
