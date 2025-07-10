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

	testCases := []struct {
		name       string
		key        string
		value      interface{}
		checkValue func(t *testing.T, res interface{}, err error, initialVal interface{})
	}{
		{
			name:  "Success - nil",
			key:   "test_key:get:nil",
			value: nil,
			checkValue: func(t *testing.T, res interface{}, err error, _ interface{}) {
				require.NoError(t, err)
				require.Nil(t, res)
			},
		},
		{
			name:  "Success - string",
			key:   "test_key:get:string",
			value: "test_val",
			checkValue: func(t *testing.T, res interface{}, err error, initialVal interface{}) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				cachedVal, ok := res.(string)
				require.True(t, ok)
				require.Equal(t, initialVal, cachedVal)
			},
		},
		{
			name:  "Success - int",
			key:   "test_key:get:int",
			value: 420,
			checkValue: func(t *testing.T, res interface{}, err error, initialVal interface{}) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				cachedVal, ok := res.(float64)
				require.True(t, ok)
				require.Equal(t, initialVal, int(cachedVal))
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			err := cache.Set(context.Background(), tc.key, tc.value, 30*time.Second)
			require.NoError(t, err)

			val, err := cache.Get(context.Background(), tc.key)
			tc.checkValue(t, val, err, tc.value)
		})
	}
}

func TestRedisCache_Get_KeyNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis cache test in short mode")
	}

	cache := newRedisCache(t)

	val, err := cache.Get(context.Background(), "test_key:not_set_in_cache")
	require.NoError(t, err)
	require.Nil(t, val)
}

func TestRedisCache_Del(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis cache test in short mode")
	}

	cache := newRedisCache(t)

	testCases := []struct {
		name             string
		key              string
		value            interface{}
		shouldSetKey     bool
		checkDeleteError func(t *testing.T, deleteErr error)
	}{
		{
			name:         "Success - nil",
			key:          "test_key:del:nil",
			value:        nil,
			shouldSetKey: true,
			checkDeleteError: func(t *testing.T, deleteErr error) {
				require.NoError(t, deleteErr)
			},
		},
		{
			name:         "Success - string",
			key:          "test_key:del:string",
			value:        "test_val",
			shouldSetKey: true,
			checkDeleteError: func(t *testing.T, deleteErr error) {
				require.NoError(t, deleteErr)
			},
		},
		{
			name:         "Success - int",
			key:          "test_key:del:int",
			value:        420,
			shouldSetKey: true,
			checkDeleteError: func(t *testing.T, deleteErr error) {
				require.NoError(t, deleteErr)
			},
		},
		{
			name:         "Success - key not in cache",
			key:          "test_key:del:not-in-cache",
			value:        "test_val",
			shouldSetKey: false,
			checkDeleteError: func(t *testing.T, deleteErr error) {
				require.NoError(t, deleteErr)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			if tc.shouldSetKey {
				err := cache.Set(context.Background(), tc.key, tc.value, 30*time.Second)
				require.NoError(t, err)

				val, getErr := cache.Get(context.Background(), tc.key)
				if tc.value != nil {
					require.NotEmpty(t, val)
				}
				require.NoError(t, getErr)
			}

			delErr := cache.Del(context.Background(), tc.key)
			tc.checkDeleteError(t, delErr)
		})
	}
}
