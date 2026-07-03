// Package ratelimit — составной ограничитель частоты запросов к Riot API.
//
// Personal Development Key даёт два лимита одновременно, и оба действуют глобально
// на весь бэкенд, а не на пользователя (SPEC.md 3.2). Значит один экземпляр Limiter
// делят между собой HTTP-хендлеры и фоновый sync worker.
//
// Лимиты считаются скользящим окном, а не токен-бакетом golang.org/x/time/rate,
// как предлагает SPEC.md 3.1. Причина: токен-бакет с burst = N доливает токены
// внутри того же окна — выпустив 20 запросов в t=0.9с, он выпустит 21-й уже в
// t=0.95с, и все они попадут в одну секунду по счётчику Riot. Скользящее окно даёт
// ровно «не больше N за любые Per» и при этом сохраняет берст после простоя.
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Limit — «не больше Requests запросов за Per».
type Limit struct {
	Requests int
	Per      time.Duration
}

// RiotDevKeyLimits — лимиты Personal Development Key (SPEC.md 3.2).
var RiotDevKeyLimits = []Limit{
	{Requests: 20, Per: time.Second},
	{Requests: 100, Per: 2 * time.Minute},
}

// ErrWouldExceedDeadline возвращается, когда очередь в лимитере заведомо длиннее
// оставшегося времени контекста. Обёрнут context.DeadlineExceeded, чтобы
// вызывающий код мог не разбирать особый случай.
var ErrWouldExceedDeadline = fmt.Errorf(
	"ratelimit: ожидание в очереди не укладывается в дедлайн контекста: %w", context.DeadlineExceeded)

// Limiter пропускает запрос, только когда его разрешают все настроенные лимиты.
// Безопасен для конкурентного использования.
type Limiter struct {
	mu      sync.Mutex
	windows []*window

	// last — момент последней выданной резервации. Коммиты не должны идти вспять:
	// на отсортированности times держится вся арифметика окна.
	last time.Time

	// now и sleep вынесены в поля, чтобы тесты могли подменить часы: иначе
	// проверка двухминутного окна Riot занимала бы две минуты.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// New собирает лимитер из набора лимитов. Пустой набор — заглушка, пропускающая всё.
//
// Некорректный лимит — ошибка программиста (лимиты приходят из констант), поэтому
// паникуем на старте, а не отдаём ошибку в рантайм.
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

// Wait блокирует вызывающего, пока запрос не разрешат все лимиты.
//
// Момент отправки выбирается сразу по всем окнам и в них же фиксируется — поэтому
// учёт не может разъехаться. Последовательное ожидание (сначала одно окно, потом
// другое) сюда не годится: быстрый лимит списал бы слот в момент t, хотя запрос
// реально уходит в t+T после разблокировки медленного.
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

// reserve выбирает ближайший момент, разрешённый всеми окнами, и занимает в них слот.
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

// rollback возвращает слот, если запрос так и не ушёл (отменённый контекст).
//
// Откатывается только последняя резервация: если за время ожидания кто-то занял
// слот после нас, вырезать наш из середины нельзя. Тогда слот остаётся занятым —
// это делает лимитер строже, а не мягче, и потому безопасно.
func (l *Limiter) rollback(at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, w := range l.windows {
		w.rollback(at)
	}
}

// window — скользящее окно «не больше limit запросов за per».
// Хранит моменты последних запросов по возрастанию; не потокобезопасно,
// синхронизация — на Limiter.
type window struct {
	limit int
	per   time.Duration
	times []time.Time
}

// earliest — ближайший момент, в который окно пропустит ещё один запрос.
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

// prune выбрасывает моменты, вышедшие из окна: они больше ни на что не влияют
// и иначе копились бы бесконечно.
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
