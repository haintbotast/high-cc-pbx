package cache

import (
	"context"
	"log"
	"time"
)

// Manager manages the cache and periodic cleanup
type Manager struct {
	cache           *LRUCache
	cleanupInterval time.Duration
	done            chan struct{}
}

// Config holds cache manager configuration
type Config struct {
	MaxEntries      int           // Maximum number of cache entries
	CleanupInterval time.Duration // How often to cleanup expired entries
}

// NewManager creates a new cache manager
func NewManager(cfg *Config) *Manager {
	if cfg.MaxEntries == 0 {
		cfg.MaxEntries = 10000 // Default: 10k entries
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 60 * time.Second // Default: 1 minute
	}

	return &Manager{
		cache:           New(cfg.MaxEntries),
		cleanupInterval: cfg.CleanupInterval,
		done:            make(chan struct{}),
	}
}

// Get retrieves a value from cache
func (m *Manager) Get(key string) (interface{}, bool) {
	return m.cache.Get(key)
}

// Set stores a value in cache with TTL
func (m *Manager) Set(key string, value interface{}, ttl time.Duration) {
	m.cache.Set(key, value, ttl)
}

// Delete removes a key from cache
func (m *Manager) Delete(key string) {
	m.cache.Delete(key)
}

// Clear removes all entries from cache
func (m *Manager) Clear() {
	m.cache.Clear()
}

// Stats returns cache statistics
func (m *Manager) Stats() CacheStats {
	return m.cache.Stats()
}

// Start begins the cache cleanup worker
func (m *Manager) Start(ctx context.Context) {
	log.Printf("[CacheManager] Starting with max_entries=%d, cleanup_interval=%v",
		m.cache.capacity, m.cleanupInterval)

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CacheManager] Shutting down...")
			close(m.done)
			return

		case <-ticker.C:
			removed := m.cache.Cleanup()
			if removed > 0 {
				log.Printf("[CacheManager] Cleaned up %d expired entries", removed)
			}

			// Log stats periodically
			stats := m.cache.Stats()
			log.Printf("[CacheManager] Stats: hits=%d, misses=%d, hit_rate=%.2f%%, size=%d/%d",
				stats.Hits, stats.Misses, stats.HitRate, stats.Size, stats.Capacity)
		}
	}
}

// Stop signals the cache manager to stop
func (m *Manager) Stop() {
	<-m.done
}
