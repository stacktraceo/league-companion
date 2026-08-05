package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Limit struct {
	Requests int
	Per      time.Duration
}

var RiotDevKeyLimits = []Limit{
	{Requests: 20, Per: time.Second},
	{Requests: 100, Per: 2 * time.Minute},
}

var ErrWouldExceedDeadline = fmt.Errorf(
	"ratelimit: ожидание в очереди не укладывается в дедлайн контекста: %w", context.DeadlineExceeded)

type Limiter struct {
	mu      sync.Mutex
	windows []*window

	// last - момент последней выданной резервации. Коммиты не должны идти вспять:
	// на отсортированности times держится вся арифметика окна.
	last time.Time

	// now и sleep вынесены в поля, чтобы тесты могли подменить часы: иначе
	// проверка двухминутного окна Riot занимала бы две минуты.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

func New(limits ...Limit) *Limiter {
	windows := make([]*window, 0, len(limits))

	for _, limit := range limits {
		if limit.Requests < 1 || limit.Per <= 0 {
			panic(fmt.Sprintf("ratelimit: некорректный лимит %d запросов за %s", limit.Requests, limit.Per))
		}

		windows = append(windows, &window{
			limit: limit.Requests,
			per:   limit.Per,
			times: make([]time.Time, 0, limit.Requests+1),
		})
	}

	return &Limiter{
		windows: windows,
		now:     time.Now,
		sleep:   sleep,
	}
}

func (l *Limiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(l.windows) == 0 {
		return nil
	}

	now := l.now()

	at, err := l.reserve(ctx, now)
	if err != nil {
		return err
	}

	delay := at.Sub(now)
	if delay <= 0 {
		return nil
	}

	if err := l.sleep(ctx, delay); err != nil {
		l.rollback(at)

		return err
	}

	return nil
}

func (l *Limiter) reserve(ctx context.Context, now time.Time) (time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	at := now

	for _, w := range l.windows {
		if earliest := w.earliest(now); earliest.After(at) {
			at = earliest
		}
	}

	if at.Before(l.last) {
		at = l.last
	}

	// Дедлайн проверяем до фиксации: спать заведомо дольше, чем живёт запрос,
	// бессмысленно, а незанятый слот не придётся возвращать.
	if deadline, ok := ctx.Deadline(); ok && at.After(deadline) {
		return time.Time{}, fmt.Errorf("%w: нужно подождать %s", ErrWouldExceedDeadline, at.Sub(now))
	}

	for _, w := range l.windows {
		w.commit(at)
	}

	l.last = at

	return at, nil
}

func (l *Limiter) rollback(at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, w := range l.windows {
		w.rollback(at)
	}
}

type window struct {
	limit int
	per   time.Duration
	times []time.Time
}

func (w *window) earliest(now time.Time) time.Time {
	w.prune(now)

	if len(w.times) < w.limit {
		return now
	}

	// Чтобы в (at-per, at] осталось не больше limit-1 прежних запросов, момент at
	// должен обогнать limit-й с конца ровно на per.
	at := w.times[len(w.times)-w.limit].Add(w.per)
	if at.Before(now) {
		return now
	}

	return at
}

func (w *window) commit(at time.Time) {
	w.times = append(w.times, at)
}

func (w *window) rollback(at time.Time) {
	if n := len(w.times); n > 0 && w.times[n-1].Equal(at) {
		w.times = w.times[:n-1]
	}
}

func (w *window) prune(now time.Time) {
	cutoff := now.Add(-w.per)

	expired := 0
	for expired < len(w.times) && !w.times[expired].After(cutoff) {
		expired++
	}

	if expired > 0 {
		w.times = append(w.times[:0], w.times[expired:]...)
	}
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
