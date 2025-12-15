package eventbus

import (
	"time"
)

const (
	defaultReadBlockDuration = 5 * time.Second
	defaultMaxRetries        = 5
	defaultRetryBaseDelay    = 100 * time.Millisecond
	defaultRetryMaxDelay     = 5 * time.Second
)

type Config struct {
	ReadBlockDuration time.Duration
	MaxRetries        int
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
}

type Option func(*Config)

func WithReadBlockDuration(duration time.Duration) Option {
	return func(c *Config) {
		c.ReadBlockDuration = duration
	}
}

func WithMaxRetries(numMaxRetries int) Option {
	return func(c *Config) {
		c.MaxRetries = numMaxRetries
	}
}

func WithRetryBaseDelay(duration time.Duration) Option {
	return func(c *Config) {
		c.RetryBaseDelay = duration
	}
}

func WithRetryMaxDelay(duration time.Duration) Option {
	return func(c *Config) {
		c.RetryMaxDelay = duration
	}
}

func (c *Config) applyDefaults() {
	if c.ReadBlockDuration == 0 {
		c.ReadBlockDuration = defaultReadBlockDuration
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultMaxRetries
	}
	if c.RetryBaseDelay == 0 {
		c.RetryBaseDelay = defaultRetryBaseDelay
	}
	if c.RetryMaxDelay == 0 {
		c.RetryMaxDelay = defaultRetryMaxDelay
	}
}
