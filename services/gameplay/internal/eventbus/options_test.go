package eventbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigApplyDefaults(t *testing.T) {
	config := Config{}
	config.applyDefaults()

	require.Equal(t, 5*time.Second, config.ReadBlockDuration)
	require.Equal(t, 5, config.MaxRetries)
	require.Equal(t, 100*time.Millisecond, config.RetryBaseDelay)
	require.Equal(t, 5*time.Second, config.RetryMaxDelay)
}

func TestConfigApplyDefaultsPreservesSetValues(t *testing.T) {
	config := Config{
		ReadBlockDuration: 10 * time.Second,
		MaxRetries:        10,
		RetryBaseDelay:    200 * time.Millisecond,
		RetryMaxDelay:     10 * time.Second,
	}
	config.applyDefaults()

	require.Equal(t, 10*time.Second, config.ReadBlockDuration)
	require.Equal(t, 10, config.MaxRetries)
	require.Equal(t, 200*time.Millisecond, config.RetryBaseDelay)
	require.Equal(t, 10*time.Second, config.RetryMaxDelay)
}

func TestConfigApplyDefaultsPartialConfig(t *testing.T) {
	config := Config{
		ReadBlockDuration: 10 * time.Second,
	}
	config.applyDefaults()

	require.Equal(t, 10*time.Second, config.ReadBlockDuration)
	require.Equal(t, 5, config.MaxRetries)
	require.Equal(t, 100*time.Millisecond, config.RetryBaseDelay)
	require.Equal(t, 5*time.Second, config.RetryMaxDelay)
}

func TestWithReadBlockDuration(t *testing.T) {
	config := Config{}
	opt := WithReadBlockDuration(3 * time.Second)
	opt(&config)

	require.Equal(t, 3*time.Second, config.ReadBlockDuration)
}

func TestWithMaxRetries(t *testing.T) {
	config := Config{}
	opt := WithMaxRetries(10)
	opt(&config)

	require.Equal(t, 10, config.MaxRetries)
}

func TestWithRetryBaseDelay(t *testing.T) {
	config := Config{}
	opt := WithRetryBaseDelay(500 * time.Millisecond)
	opt(&config)

	require.Equal(t, 500*time.Millisecond, config.RetryBaseDelay)
}

func TestWithRetryMaxDelay(t *testing.T) {
	config := Config{}
	opt := WithRetryMaxDelay(30 * time.Second)
	opt(&config)

	require.Equal(t, 30*time.Second, config.RetryMaxDelay)
}

func TestMultipleOptions(t *testing.T) {
	config := Config{}

	opts := []Option{
		WithReadBlockDuration(2 * time.Second),
		WithMaxRetries(7),
		WithRetryBaseDelay(50 * time.Millisecond),
		WithRetryMaxDelay(10 * time.Second),
	}

	for _, opt := range opts {
		opt(&config)
	}

	require.Equal(t, 2*time.Second, config.ReadBlockDuration)
	require.Equal(t, 7, config.MaxRetries)
	require.Equal(t, 50*time.Millisecond, config.RetryBaseDelay)
	require.Equal(t, 10*time.Second, config.RetryMaxDelay)
}
