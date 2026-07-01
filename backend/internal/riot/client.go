// Package riot — клиент к Riot Games API: Account-V1, Summoner-V4, League-V4, Match-V5.
//
// Клиент отвечает за построение URL (через routing.go), заголовок X-Riot-Token
// и классификацию ответов в типизированные ошибки. Rate limiting, retry/backoff
// и кэш навешиваются поверх отдельными слоями (SPEC.md 3.2).
package riot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// apiKeyHeader — заголовок аутентификации Riot API.
	apiKeyHeader = "X-Riot-Token"

	// maxErrorBody — сколько байт тела читать у неуспешного ответа для диагностики.
	maxErrorBody = 4 << 10

	// maxResponseBody — потолок на успешный ответ. Полный Match-V5 занимает
	// сотни килобайт, так что запас нужен ощутимый.
	maxResponseBody = 16 << 20

	// defaultRetryAfter — на что заменить Retry-After, если Riot его не прислал.
	defaultRetryAfter = time.Second

	defaultTimeout = 10 * time.Second
)

// Client — HTTP-клиент к Riot API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger

	// baseURL переопределяет схему и хост во всех запросах. Используется только
	// в тестах (httptest), в бою пустой.
	baseURL string
}

// Option — функциональная опция конструктора.
type Option func(*Client)

// WithHTTPClient подменяет http.Client — например, чтобы навесить транспорт
// с rate limiter'ом.
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

// New создаёт клиент с ключом Riot API.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// get выполняет GET-запрос и, если out != nil, разбирает тело в out.
// Всегда возвращает сырое тело успешного ответа — оно нужно для matches.raw_data.
func (c *Client) get(ctx context.Context, host, path string, query url.Values, out any) (json.RawMessage, error) {
	endpoint := c.buildURL(host, path, query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось собрать запрос к %s: %w", path, err)
	}

	req.Header.Set(apiKeyHeader, c.apiKey)
	req.Header.Set("Accept", "application/json")

	started := time.Now()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос к Riot API (%s) не удался: %w", path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	c.logger.DebugContext(ctx, "riot request",
		"path", path,
		"host", host,
		"status", resp.StatusCode,
		"duration", time.Since(started),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, classifyError(resp, path)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать ответ Riot API (%s): %w", path, err)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, fmt.Errorf("не удалось разобрать ответ Riot API (%s): %w", path, err)
		}
	}

	return body, nil
}

// buildURL собирает полный URL запроса. Хост приходит из routing.go — руками
// здесь ничего не склеивается (SPEC.md 7).
func (c *Client) buildURL(host, path string, query url.Values) string {
	base := "https://" + host
	if c.baseURL != "" {
		base = c.baseURL
	}

	endpoint := base + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	return endpoint
}

// classifyError переводит неуспешный ответ Riot в типизированную ошибку.
func classifyError(resp *http.Response, path string) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, path)

	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w (%d на %s)", ErrUnauthorized, resp.StatusCode, path)

	case http.StatusTooManyRequests:
		return &RateLimitError{
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			Scope:      resp.Header.Get("X-Rate-Limit-Type"),
			Endpoint:   path,
		}

	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))

		return &UpstreamError{
			StatusCode: resp.StatusCode,
			Endpoint:   path,
			Body:       strings.TrimSpace(string(body)),
		}
	}
}

// parseRetryAfter разбирает заголовок Retry-After (Riot присылает целые секунды).
// Отсутствующее или мусорное значение — не повод игнорировать паузу, поэтому
// откатываемся к defaultRetryAfter.
func parseRetryAfter(header string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || seconds <= 0 {
		return defaultRetryAfter
	}

	return time.Duration(seconds) * time.Second
}
