package riot

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound - Riot вернул 404: саммонера/матча не существует.
	ErrNotFound = errors.New("riot: не найдено")

	// ErrUnauthorized - Riot вернул 401/403. На Development Key это почти всегда
	// значит, что ключ протух: он живёт 24 часа и перевыпускается вручную
	// на developer.riotgames.com (SPEC.md 3.2).
	ErrUnauthorized = errors.New("riot: ключ отклонён (проверь RIOT_API_KEY - dev-ключ живёт 24 часа)")
)

type RateLimitError struct {
	// RetryAfter - сколько ждать до следующей попытки.
	RetryAfter time.Duration

	// Scope - значение X-Rate-Limit-Type: application, method или service.
	Scope string

	// Endpoint - путь запроса, без ключа и хоста.
	Endpoint string
}

func (e *RateLimitError) Error() string {
	scope := e.Scope
	if scope == "" {
		scope = "unknown"
	}

	return fmt.Sprintf("riot: превышен лимит запросов (%s) на %s, повтор через %s", scope, e.Endpoint, e.RetryAfter)
}

type UpstreamError struct {
	StatusCode int
	Endpoint   string
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("riot: неожиданный ответ %d на %s: %s", e.StatusCode, e.Endpoint, e.Body)
}

func RetryAfter(err error) (time.Duration, bool) {
	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return rateLimitErr.RetryAfter, true
	}

	return 0, false
}

func IsRetryable(err error) bool {
	if _, ok := RetryAfter(err); ok {
		return true
	}

	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode >= 500
	}

	return false
}
