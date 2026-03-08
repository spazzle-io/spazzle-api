package storage

import (
	"context"
	"errors"
	"io"
)

var ErrObjectNotFound = errors.New("object not found in store")

// ObjectStore defines a generic interface for object storage
type ObjectStore interface {
	// Put stores data under the given key in the specified bucket.
	Put(ctx context.Context, bucket string, key string, data io.Reader, contentType string) error
	// Get retrieves the object stored under the given key.
	// Returns ErrObjectNotFound if the key does not exist.
	Get(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	// Delete removes the object stored under the given key.
	// It is a no-op if the key does not exist.
	Delete(ctx context.Context, bucket string, key string) error
	// Close closes the store and releases any open resources.
	Close() error
}
