package riot

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Waiter interface {
	Wait(ctx context.Context) error
}

type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = timeout }
}

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) { c.logger = logger }
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimSuffix(baseURL, "/") }
}

func WithRateLimiter(limiter Waiter) Option {
	return func(c *Client) { c.limiter = limiter }
}

func WithCache(cache Cache) Option {
	return func(c *Client) { c.cache = cache }
}

func WithRetry(attempts int, base time.Duration) Option {
	return func(c *Client) {
		if attempts < 1 {
			attempts = 1
		}

		c.retryAttempts = attempts
		c.retryBase = base
	}
}
