// Package riot — клиент к Riot Games API: Account-V1, Summoner-V4, League-V4, Match-V5.
//
// Клиент отвечает за построение URL (через routing.go), заголовок X-Riot-Token
// и классификацию ответов в типизированные ошибки. Поверх этого он проводит каждый
// запрос через кэш, общий ограничитель частоты и backoff на 429/5xx (SPEC.md 3.2).
package riot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
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

	// Параметры повторов по умолчанию.
	defaultRetryAttempts = 3
	defaultRetryBase     = 500 * time.Millisecond

	// maxRetryDelay — потолок паузы между попытками, кроме случая, когда Riot
	// сам назвал Retry-After: его слушаемся как есть.
	maxRetryDelay = 30 * time.Second
)

// Время жизни закэшированных ответов.
//
// Детали матча не кэшируются осознанно: они неизменяемы, весят сотни килобайт и
// целиком ложатся в matches.raw_data (DECISIONS.md, отклонение 1) — Postgres и есть
// их кэш, дублировать его в Redis незачем.
const (
	accountTTL  = 24 * time.Hour
	summonerTTL = 10 * time.Minute
	leagueTTL   = 5 * time.Minute
	matchIDsTTL = time.Minute
	matchTTL    = 0
)

// Client — HTTP-клиент к Riot API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger

	// baseURL переопределяет схему и хост во всех запросах. Используется только
	// в тестах (httptest), в бою пустой.
	baseURL string

	limiter Waiter
	cache   Cache

	retryAttempts int
	retryBase     time.Duration

	// sleep вынесен в поле, чтобы тесты на backoff не ждали настоящие секунды.
	sleep func(ctx context.Context, d time.Duration) error
}

// New создаёт клиент с ключом Riot API.
//
// По умолчанию клиент ходит в Riot без ограничителя и без кэша — и то и другое
// подключается опциями, потому что делится на весь процесс.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:        apiKey,
		httpClient:    &http.Client{Timeout: defaultTimeout},
		logger:        slog.Default(),
		retryAttempts: defaultRetryAttempts,
		retryBase:     defaultRetryBase,
		sleep:         sleep,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// request — описание одного обращения к Riot.
type request struct {
	host  string
	path  string
	query url.Values

	// ttl — сколько хранить ответ в кэше; 0 отключает кэширование для вызова.
	ttl time.Duration

	// out — куда разобрать тело; nil, если разбор не нужен.
	out any
}

// cacheKey однозначно определяет ответ. Ключ Riot сюда не попадает: он живёт
// в заголовке, а не в URL (DECISIONS.md, «Конвенции»).
func (r request) cacheKey() string {
	key := r.host + r.path
	if len(r.query) > 0 {
		key += "?" + r.query.Encode()
	}

	return key
}

// do проводит запрос через кэш, ограничитель и повторы и возвращает сырое тело.
//
// Порядок здесь принципиален: кэш проверяется до ограничителя (попадание не должно
// тратить бюджет Riot), а ограничитель дёргается внутри цикла повторов (повтор —
// такой же запрос к Riot, как и первая попытка).
func (c *Client) do(ctx context.Context, req request) (json.RawMessage, error) {
	if body, ok := c.fromCache(ctx, req); ok {
		return body, nil
	}

	body, err := c.fetch(ctx, req)
	if err != nil {
		return nil, err
	}

	c.toCache(ctx, req, body)

	return body, nil
}

// fromCache отдаёт разобранное значение из кэша. Любая проблема — недоступный
// Redis, битое значение — это не отказ, а просто поход в Riot.
func (c *Client) fromCache(ctx context.Context, req request) (json.RawMessage, bool) {
	if c.cache == nil || req.ttl <= 0 {
		return nil, false
	}

	key := req.cacheKey()

	body, ok, err := c.cache.Get(ctx, key)
	if err != nil {
		c.logger.WarnContext(ctx, "кэш недоступен на чтение, иду в Riot", "path", req.path, "error", err)

		return nil, false
	}

	if !ok {
		return nil, false
	}

	if req.out != nil {
		if err := json.Unmarshal(body, req.out); err != nil {
			c.logger.WarnContext(ctx, "испорченное значение в кэше, иду в Riot", "path", req.path, "error", err)

			return nil, false
		}
	}

	c.logger.DebugContext(ctx, "riot cache hit", "path", req.path, "host", req.host)

	return body, true
}

func (c *Client) toCache(ctx context.Context, req request, body json.RawMessage) {
	if c.cache == nil || req.ttl <= 0 {
		return
	}

	if err := c.cache.Set(ctx, req.cacheKey(), body, req.ttl); err != nil {
		c.logger.WarnContext(ctx, "не удалось записать ответ в кэш", "path", req.path, "error", err)
	}
}

// fetch выполняет запрос с повторами на 429 и 5xx.
func (c *Client) fetch(ctx context.Context, req request) (json.RawMessage, error) {
	var lastErr error

	for attempt := range c.retryAttempts {
		if attempt > 0 {
			if err := c.backoff(ctx, req, lastErr, attempt); err != nil {
				return nil, err
			}
		}

		// Ограничитель — внутри цикла: повторная попытка тоже расходует лимит Riot.
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("riot: ожидание лимитера перед %s: %w", req.path, err)
			}
		}

		body, err := c.get(ctx, req.host, req.path, req.query, req.out)
		if err == nil {
			return body, nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

// backoff выдерживает паузу перед повторной попыткой attempt (нумерация с 1).
func (c *Client) backoff(ctx context.Context, req request, lastErr error, attempt int) error {
	delay := c.retryDelay(lastErr, attempt-1)

	// Ждать дольше, чем живёт запрос, бессмысленно — отдаём последнюю ошибку сразу.
	if deadline, ok := ctx.Deadline(); ok && time.Now().Add(delay).After(deadline) {
		return lastErr
	}

	c.logger.WarnContext(ctx, "повторяю запрос к Riot",
		"path", req.path,
		"attempt", attempt+1,
		"of", c.retryAttempts,
		"delay", delay,
		"error", lastErr,
	)

	if err := c.sleep(ctx, delay); err != nil {
		return errors.Join(lastErr, err)
	}

	return nil
}

// retryDelay выбирает паузу перед следующей попыткой.
//
// На 429 Riot сам называет срок в Retry-After — его и выдерживаем, самодеятельность
// здесь только злит лимитер (SPEC.md 3.2). На 5xx — экспонента с джиттером: без
// него пул воркеров вехи 7, огребший 503, пойдёт на повтор синхронно.
func (c *Client) retryDelay(err error, exponent int) time.Duration {
	if retryAfter, ok := RetryAfter(err); ok {
		return retryAfter
	}

	delay := c.retryBase << exponent
	if delay > maxRetryDelay || delay <= 0 {
		delay = maxRetryDelay
	}

	half := delay / 2

	return half + rand.N(half+1) //nolint:gosec // джиттер, криптостойкость не нужна
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

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
