package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testRedisConnUrl = "redis://0.0.0.0:6379"

func newRedisCache(t *testing.T) Cache {
	redisCache, err := NewRedisCache(testRedisConnUrl)
	require.NoError(t, err)
	require.NotEmpty(t, redisCache)

	return redisCache
}

func TestRedisCache_Set(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis cache test in short mode")
	}

	cache := newRedisCache(t)

	testCases := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "Success - nil",
			key:   "test_key:set:nil",
			value: nil,
		},
		{
			name:  "Success - string",
			key:   "test_key:set:string",
			value: "test_val",
		},
		{
			name:  "Success - int",
			key:   "test_key:set:int",
			value: 420,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			err := cache.Set(context.Background(), tc.key, tc.value, 30*time.Second)
			require.NoError(t, err)
		})
	}
}

func TestRedisCache_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis cache test in short mode")
	}

	cache := newRedisCache(t)

	t.Run("Success - not found", func(t *testing.T) {
		var result string
		err := cache.Get(context.Background(), "test_key:get:notfound", &result)
		require.ErrorIs(t, err, ErrKeyNotFound)
		require.Empty(t, result)
	})

	t.Run("Success - string", func(t *testing.T) {
		key := "test_key:get:string"
		expected := "test_val"

		err := cache.Set(context.Background(), key, expected, 30*time.Second)
		require.NoError(t, err)

		var result string
		err = cache.Get(context.Background(), key, &result)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("Success - int", func(t *testing.T) {
		key := "test_key:get:int"
		expected := 420

		err := cache.Set(context.Background(), key, expected, 30*time.Second)
		require.NoError(t, err)

		var result int
		err = cache.Get(context.Background(), key, &result)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("Success - slice", func(t *testing.T) {
		key := "test_key:get:slice"
		expected := []string{"a", "b", "c"}

		err := cache.Set(context.Background(), key, expected, 30*time.Second)
		require.NoError(t, err)

		var result []string
		err = cache.Get(context.Background(), key, &result)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("Success - struct", func(t *testing.T) {
		key := "test_key:get:struct"
		type testStruct struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		expected := testStruct{Name: "test", Count: 42}

		err := cache.Set(context.Background(), key, expected, 30*time.Second)
		require.NoError(t, err)

		var result testStruct
		err = cache.Get(context.Background(), key, &result)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})
}

func TestRedisCache_Del(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis cache test in short mode")
	}

	cache := newRedisCache(t)

	t.Run("Success - key exists", func(t *testing.T) {
		key := "test_key:del:exists"

		err := cache.Set(context.Background(), key, "test_val", 30*time.Second)
		require.NoError(t, err)

		err = cache.Del(context.Background(), key)
		require.NoError(t, err)

		var result string
		err = cache.Get(context.Background(), key, &result)
		require.ErrorIs(t, err, ErrKeyNotFound)
		require.Empty(t, result)
	})

	t.Run("Success - key does not exist", func(t *testing.T) {
		err := cache.Del(context.Background(), "test_key:del:notfound")
		require.NoError(t, err)
	})
}
