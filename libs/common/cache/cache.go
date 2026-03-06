package cache

import (
	"context"
	"errors"
	"time"
)

var ErrKeyNotFound = errors.New("key not found in cache")

// Cache defines a generic interface for a key-value cache
type Cache interface {
	// Set stores a value associated with the given key in the cache.
	// The value will expire and be automatically removed after the specified duration.
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	// Get retrieves the value associated with the given key from the cache.
	// If the key is not present in the cache, it returns ErrKeyNotFound.
	Get(ctx context.Context, key string, dest interface{}) error
	// Del removes the value associated with the given key from the cache.
	// It is a no-op if the key does not exist.
	Del(ctx context.Context, key string) error
	// Close closes the cache and releases any open resources.
	Close() error
}
