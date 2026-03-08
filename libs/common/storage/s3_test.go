package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping storage test in short mode")
	}

	ctx := context.Background()
	key := fmt.Sprintf("test/%s.json", uuid.New().String())
	data := []byte(`{"key":"val","foo":bar}`)

	err := testStore.Put(ctx, testBucket, key, bytes.NewReader(data), "application/json")
	require.NoError(t, err)

	reader, err := testStore.Get(ctx, testBucket, key)
	require.NoError(t, err)
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		require.NoError(t, err)
	}(reader)

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestGetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping storage test in short mode")
	}

	ctx := context.Background()
	key := fmt.Sprintf("test/%s.json", uuid.New().String())

	_, err := testStore.Get(ctx, testBucket, key)
	assert.ErrorIs(t, err, ErrObjectNotFound)
}

func TestDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping storage test in short mode")
	}

	ctx := context.Background()
	key := fmt.Sprintf("test/%s.json", uuid.New().String())
	data := []byte(`{"foo":"bar","baz":50}`)

	err := testStore.Put(ctx, testBucket, key, bytes.NewReader(data), "application/json")
	require.NoError(t, err)

	err = testStore.Delete(ctx, testBucket, key)
	require.NoError(t, err)

	_, err = testStore.Get(ctx, testBucket, key)
	assert.ErrorIs(t, err, ErrObjectNotFound)
}

func TestDeleteNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping storage test in short mode")
	}

	ctx := context.Background()
	key := fmt.Sprintf("test/%s.json", uuid.New().String())

	err := testStore.Delete(ctx, testBucket, key)
	assert.NoError(t, err)
}
