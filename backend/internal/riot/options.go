package riot

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Waiter — ограничитель частоты запросов к Riot.
//
// Лимиты Riot глобальны на весь ключ (SPEC.md 3.2), поэтому сюда должен приходить
// один экземпляр на весь процесс — общий у HTTP-хендлеров и sync worker'а.
// Создавать лимитер на каждый клиент нельзя: лимит перестанет соблюдаться.
type Waiter interface {
	Wait(ctx context.Context) error
}

// Cache — кэш сырых ответов Riot. Необязателен: промах или ошибка означают
// только поход в Riot.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Option — функциональная опция конструктора.
type Option func(*Client)

// WithHTTPClient подменяет http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// WithTimeout задаёт таймаут одного запроса.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = timeout }
}

// WithLogger задаёт логгер.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) { c.logger = logger }
}

// WithBaseURL направляет все запросы на указанный базовый URL вместо хостов
// Riot. Предназначено для тестов.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimSuffix(baseURL, "/") }
}

// WithRateLimiter подключает ограничитель частоты. Без него клиент шлёт запросы
// без оглядки на лимиты Riot — годится только для тестов.
func WithRateLimiter(limiter Waiter) Option {
	return func(c *Client) { c.limiter = limiter }
}

// WithCache подключает кэш ответов. Без него каждый вызов идёт в Riot.
func WithCache(cache Cache) Option {
	return func(c *Client) { c.cache = cache }
}

// WithRetry задаёт число попыток (включая первую) и базовую паузу backoff'а.
// attempts < 1 трактуется как 1 — то есть без повторов.
func WithRetry(attempts int, base time.Duration) Option {
	return func(c *Client) {
		if attempts < 1 {
			attempts = 1
		}

		c.retryAttempts = attempts
		c.retryBase = base
	}
}
