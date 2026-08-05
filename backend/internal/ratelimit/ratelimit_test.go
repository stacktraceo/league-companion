package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	slept   time.Duration
	onSleep func()
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Now()}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if c.onSleep != nil {
		c.onSleep()
	}

	// Отмена, случившаяся во время ожидания.
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
	c.slept += d

	return nil
}

func (c *fakeClock) Slept() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.slept
}

func (c *fakeClock) ResetSlept() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.slept = 0
}

func newTestLimiter(clock *fakeClock, limits ...Limit) *Limiter {
	limiter := New(limits...)
	limiter.now = clock.Now
	limiter.sleep = clock.Sleep

	return limiter
}

func waitTimes(t *testing.T, limiter *Limiter, clock *fakeClock, n int) []time.Time {
	t.Helper()

	times := make([]time.Time, 0, n)

	for i := range n {
		require.NoErrorf(t, limiter.Wait(context.Background()), "запрос %d", i+1)
		times = append(times, clock.Now())
	}

	return times
}

func assertRespectsLimits(t *testing.T, times []time.Time, limits ...Limit) {
	t.Helper()

	for _, limit := range limits {
		for _, start := range times {
			end := start.Add(limit.Per)
			count := 0

			for _, ts := range times {
				if !ts.Before(start) && ts.Before(end) {
					count++
				}
			}

			assert.LessOrEqualf(t, count, limit.Requests,
				"в окне %s начиная с +%s прошло %d запросов при лимите %d",
				limit.Per, start.Sub(times[0]), count, limit.Requests)
		}
	}
}

func TestRiotDevKeyLimitsMatchSpec(t *testing.T) {
	// SPEC.md 3.2: Personal Development Key - 20 запросов/сек и 100 запросов/2 минуты.
	assert.Equal(t, []Limit{
		{Requests: 20, Per: time.Second},
		{Requests: 100, Per: 2 * time.Minute},
	}, RiotDevKeyLimits)
}

func TestBurstPassesThenThrottles(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock, Limit{Requests: 20, Per: time.Second})

	waitTimes(t, limiter, clock, 20)
	assert.Zero(t, clock.Slept(), "первые 20 запросов укладываются в берст и не ждут")

	require.NoError(t, limiter.Wait(context.Background()))
	assert.Equal(t, time.Second, clock.Slept(),
		"окно скользящее: 21-й ждёт, пока первый выйдет из секундного окна, а не долива токена")
}

func TestCompositeObeysBothLimits(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock, RiotDevKeyLimits...)

	start := clock.Now()
	times := waitTimes(t, limiter, clock, 120)

	assertRespectsLimits(t, times, RiotDevKeyLimits...)

	// Первые 100 расходятся пачками по 20 в секунду (t=0..4с) и выбирают
	// двухминутный лимит целиком; следующие 20 ждут, пока самый первый запрос
	// выйдет из окна 2 минут.
	elapsed := clock.Now().Sub(start)
	assert.Equal(t, 2*time.Minute, elapsed,
		"после исчерпания лимита 100/2мин темп задаёт он, а не 20/с")
}

func TestNoTokenDriftBetweenLimiters(t *testing.T) {
	clock := newFakeClock()
	limits := []Limit{
		{Requests: 3, Per: time.Second},
		{Requests: 5, Per: 10 * time.Second},
	}
	limiter := newTestLimiter(clock, limits...)

	times := waitTimes(t, limiter, clock, 15)

	assertRespectsLimits(t, times, limits...)
}

func TestWaitWithoutLimitsIsPassThrough(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock)

	for range 1000 {
		require.NoError(t, limiter.Wait(context.Background()))
	}

	assert.Zero(t, clock.Slept())
}

func TestWaitReturnsErrorOnAlreadyCancelledContext(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock, Limit{Requests: 1, Per: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.ErrorIs(t, limiter.Wait(ctx), context.Canceled)

	// Отменённый вызов не должен был тратить берст.
	require.NoError(t, limiter.Wait(context.Background()))
	assert.Zero(t, clock.Slept())
}

func TestCancelDuringWaitReturnsToken(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock, Limit{Requests: 1, Per: time.Second})

	// Съедаем берст, чтобы следующий вызов ушёл в ожидание.
	require.NoError(t, limiter.Wait(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	clock.onSleep = cancel

	assert.ErrorIs(t, limiter.Wait(ctx), context.Canceled)
	assert.Zero(t, clock.Slept(), "отменённое ожидание не должно двигать часы")

	// Резервация отменена, токен вернулся: следующий запрос ждёт ровно секунду
	// (пополнение после первого), а не две.
	clock.onSleep = nil

	require.NoError(t, limiter.Wait(context.Background()))
	assert.Equal(t, time.Second, clock.Slept())
}

func TestWaitFailsFastWhenDelayExceedsDeadline(t *testing.T) {
	clock := newFakeClock()
	limiter := newTestLimiter(clock, Limit{Requests: 1, Per: time.Minute})

	require.NoError(t, limiter.Wait(context.Background()))

	ctx, cancel := context.WithDeadline(context.Background(), clock.Now().Add(time.Second))
	defer cancel()

	err := limiter.Wait(ctx)
	require.ErrorIs(t, err, ErrWouldExceedDeadline)
	assert.ErrorIs(t, err, context.DeadlineExceeded,
		"вызывающему коду удобно обрабатывать это как обычный дедлайн")
	assert.Zero(t, clock.Slept(), "спать заведомо дольше дедлайна бессмысленно")

	// Токен вернулся - неудачная попытка не съела бюджет.
	require.NoError(t, limiter.Wait(context.Background()))
	assert.Equal(t, time.Minute, clock.Slept())
}

func TestNewPanicsOnInvalidLimit(t *testing.T) {
	assert.Panics(t, func() { New(Limit{Requests: 0, Per: time.Second}) })
	assert.Panics(t, func() { New(Limit{Requests: 5, Per: 0}) })
}

func TestConcurrentWaitRespectsRate(t *testing.T) {
	const (
		window   = 100 * time.Millisecond
		requests = 5
		total    = 20
	)

	limiter := New(Limit{Requests: requests, Per: window})

	var wg sync.WaitGroup

	started := time.Now()
	errs := make(chan error, total)

	for range total {
		wg.Add(1)

		go func() {
			defer wg.Done()

			errs <- limiter.Wait(context.Background())
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	// 5 уходят сразу, остальные 15 - по одному каждые window/requests = 20 мс.
	elapsed := time.Since(started)
	assert.GreaterOrEqual(t, elapsed, 15*window/requests-5*time.Millisecond)
}
