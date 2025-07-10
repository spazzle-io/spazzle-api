package cache

import (
	"context"
	"time"
)

// Cache defines a generic interface for a key-value cache
type Cache interface {
	// Set stores a value associated with the given key in the cache.
	// The value will expire and be automatically removed after the specified duration.
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	// Get retrieves the value associated with the given key from the cache.
	// If the key is not found, it should return (nil, nil).
	Get(ctx context.Context, key string) (interface{}, error)
	// Del removes the value associated with the given key from the cache.
	// It is a no-op if the key does not exist.
	Del(ctx context.Context, key string) error
	// Close closes the cache and releases any open resources.
	Close() error
}
