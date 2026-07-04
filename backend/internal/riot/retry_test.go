package riot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSleeper подменяет ожидание между попытками: тесты на backoff не должны
// занимать столько же, сколько занимает сам backoff.
type fakeSleeper struct {
	mu     sync.Mutex
	delays []time.Duration
	err    error
}

func (s *fakeSleeper) Sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.delays = append(s.delays, d)

	return s.err
}

func (s *fakeSleeper) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]time.Duration(nil), s.delays...)
}

// withFakeSleep подменяет сон клиента и возвращает записанные паузы.
func withFakeSleep(client *Client) *fakeSleeper {
	sleeper := &fakeSleeper{}
	client.sleep = sleeper.Sleep

	return sleeper
}

// countingWaiter — заглушка ограничителя: считает, сколько раз его спросили.
type countingWaiter struct {
	calls atomic.Int64
	err   error
}

func (w *countingWaiter) Wait(context.Context) error {
	w.calls.Add(1)

	return w.err
}

// statusSequence отдаёт заданные коды по очереди, последний повторяется.
func statusSequence(calls *atomic.Int64, statuses ...int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		index := int(calls.Add(1)) - 1
		if index >= len(statuses) {
			index = len(statuses) - 1
		}

		status := statuses[index]
		if status == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"puuid":"`+testPUUID+`"}`)

			return
		}

		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "2")
		}

		w.WriteHeader(status)
	}
}

func TestRetriesOn429AndHonoursRetryAfter(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t,
		statusSequence(&calls, http.StatusTooManyRequests, http.StatusOK),
		WithRetry(3, 500*time.Millisecond))
	sleeper := withFakeSleep(client)

	account, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
	require.NoError(t, err)
	assert.Equal(t, testPUUID, account.PUUID)

	assert.Equal(t, int64(2), calls.Load(), "одна неудачная попытка и одна успешная")
	assert.Equal(t, []time.Duration{2 * time.Second}, sleeper.Delays(),
		"на 429 ждём ровно столько, сколько попросил Riot, без джиттера")
}

func TestRetriesOn5xxWithExponentialBackoff(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t,
		statusSequence(&calls,
			http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK),
		WithRetry(3, time.Second))
	sleeper := withFakeSleep(client)

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
	require.NoError(t, err)

	assert.Equal(t, int64(3), calls.Load())

	delays := sleeper.Delays()
	require.Len(t, delays, 2)

	// Джиттер держит паузу в пределах [base/2, base] от экспоненты.
	assert.GreaterOrEqual(t, delays[0], 500*time.Millisecond)
	assert.LessOrEqual(t, delays[0], time.Second)
	assert.GreaterOrEqual(t, delays[1], time.Second)
	assert.LessOrEqual(t, delays[1], 2*time.Second)
}

func TestRetriesStopAtAttemptLimit(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t,
		statusSequence(&calls, http.StatusServiceUnavailable),
		WithRetry(3, time.Millisecond))
	withFakeSleep(client)

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")

	var upstreamErr *UpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	assert.Equal(t, http.StatusServiceUnavailable, upstreamErr.StatusCode)
	assert.Equal(t, int64(3), calls.Load(), "ровно столько попыток, сколько задано")
}

func TestNoRetryOnNotFoundOrUnauthorized(t *testing.T) {
	for name, status := range map[string]int{
		"404": http.StatusNotFound,
		"401": http.StatusUnauthorized,
		"403": http.StatusForbidden,
		"400": http.StatusBadRequest,
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int64

			client := newTestClient(t, statusSequence(&calls, status), WithRetry(3, time.Millisecond))
			withFakeSleep(client)

			_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
			require.Error(t, err)
			assert.Equal(t, int64(1), calls.Load(), "такие ответы повторять бессмысленно")
		})
	}
}

func TestLimiterIsAskedOnEveryAttempt(t *testing.T) {
	var calls atomic.Int64

	limiter := &countingWaiter{}
	client := newTestClient(t,
		statusSequence(&calls, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK),
		WithRetry(3, time.Millisecond), WithRateLimiter(limiter))
	withFakeSleep(client)

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
	require.NoError(t, err)

	assert.Equal(t, int64(3), calls.Load())
	assert.Equal(t, int64(3), limiter.calls.Load(),
		"повтор — такой же запрос к Riot, лимит он тоже расходует")
}

func TestLimiterErrorAbortsRequest(t *testing.T) {
	var calls atomic.Int64

	limiter := &countingWaiter{err: context.DeadlineExceeded}
	client := newTestClient(t, statusSequence(&calls, http.StatusOK), WithRateLimiter(limiter))

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Zero(t, calls.Load(), "не дождавшись лимитера, в Riot не идём")
}

func TestBackoffStopsWhenContextIsDone(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t,
		statusSequence(&calls, http.StatusServiceUnavailable),
		WithRetry(3, time.Millisecond))

	sleeper := &fakeSleeper{err: context.Canceled}
	client.sleep = sleeper.Sleep

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")
	require.Error(t, err)

	assert.ErrorIs(t, err, context.Canceled)

	var upstreamErr *UpstreamError
	assert.ErrorAs(t, err, &upstreamErr, "причина отказа должна остаться видна")
	assert.Equal(t, int64(1), calls.Load())
}

// Пауза длиннее оставшегося времени контекста бессмысленна: лучше сразу отдать
// последнюю ошибку, чем проспать весь дедлайн и отдать ту же самую.
func TestBackoffSkippedWhenLongerThanDeadline(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t,
		statusSequence(&calls, http.StatusTooManyRequests),
		WithRetry(3, time.Millisecond))
	sleeper := withFakeSleep(client)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetAccountByRiotID(ctx, "ru", "Name", "TAG")

	var rateLimitErr *RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	assert.Equal(t, 2*time.Second, rateLimitErr.RetryAfter)
	assert.Empty(t, sleeper.Delays(), "спать дольше дедлайна не начинаем")
	assert.Equal(t, int64(1), calls.Load())
}

func TestRetryDelayNeverExceedsCap(t *testing.T) {
	client := New(testAPIKey, WithRetry(20, time.Second))

	for exponent := range 20 {
		delay := client.retryDelay(errors.New("boom"), exponent)
		assert.LessOrEqual(t, delay, maxRetryDelay, "экспонента не должна переполняться")
		assert.Positive(t, delay)
	}
}
