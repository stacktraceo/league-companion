package syncer

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultSyncInterval = 10 * time.Minute

type TrackedStore interface {
	TrackedPUUIDs(ctx context.Context) (map[string]struct{}, error)
}

type BatchQueue interface {
	Submit(puuid string, count int, done func()) bool
}

type Ticker struct {
	interval  time.Duration
	summoners TrackedStore
	queue     BatchQueue
	logger    *slog.Logger

	// running - guard от наложения прогонов (DECISIONS.md, отклонение 4). Цикл вызывает
	// runOnce последовательно, поэтому за пропуск тика во время долгого прогона
	// отвечает dropPendingTick; этот флаг закрывает второй путь - вызов runOnce
	// откуда-то ещё, помимо цикла.
	running atomic.Bool

	started  atomic.Bool
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

func NewTicker(
	summoners TrackedStore,
	queue BatchQueue,
	interval time.Duration,
	logger *slog.Logger,
) *Ticker {
	if interval <= 0 {
		interval = DefaultSyncInterval
	}

	return &Ticker{
		interval:  interval,
		summoners: summoners,
		queue:     queue,
		logger:    logger,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func (t *Ticker) Start(ctx context.Context) {
	t.started.Store(true)

	go t.loop(ctx)

	t.logger.InfoContext(ctx, "периодическая синхронизация запущена", "интервал", t.interval)
}

func (t *Ticker) Stop() {
	t.stopOnce.Do(func() { close(t.stop) })

	// Без Start канал done никто не закроет - ждать было бы вечно.
	if t.started.Load() {
		<-t.done
	}
}

func (t *Ticker) loop(ctx context.Context) {
	defer close(t.done)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			t.logger.InfoContext(ctx, "периодическая синхронизация остановлена")

			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.runOnce(ctx)
			t.dropPendingTick(ctx, ticker)
		}
	}
}

func (t *Ticker) dropPendingTick(ctx context.Context, ticker *time.Ticker) {
	select {
	case <-ticker.C:
		t.logger.WarnContext(ctx, "предыдущий прогон синхронизации ещё шёл, тик пропущен",
			"интервал", t.interval)
	default:
	}
}

func (t *Ticker) runOnce(ctx context.Context) int {
	// CAS, а не проверка с последующей установкой: два тика не могут проскочить
	// одновременно.
	if !t.running.CompareAndSwap(false, true) {
		t.logger.WarnContext(ctx, "предыдущий прогон синхронизации ещё идёт, тик пропущен")

		return 0
	}
	defer t.running.Store(false)

	tracked, err := t.summoners.TrackedPUUIDs(ctx)
	if err != nil {
		t.logger.ErrorContext(ctx, "не удалось получить список отслеживаемых саммонеров", "error", err)

		return 0
	}

	if len(tracked) == 0 {
		t.logger.DebugContext(ctx, "отслеживаемых саммонеров нет, синхронизировать нечего")

		return 0
	}

	started := time.Now()

	var (
		wg      sync.WaitGroup
		queued  int
		dropped int
	)

	for puuid := range tracked {
		wg.Add(1)

		if !t.queue.Submit(puuid, DefaultMatchCount, wg.Done) {
			// Задача не принята - Submit не позовёт done, поэтому снимаем счётчик сами.
			wg.Done()

			dropped++

			continue
		}

		queued++
	}

	t.waitBatch(ctx, &wg)

	t.logger.InfoContext(ctx, "прогон периодической синхронизации завершён",
		"саммонеров", queued, "отброшено", dropped, "duration", time.Since(started))

	return queued
}

func (t *Ticker) waitBatch(ctx context.Context, wg *sync.WaitGroup) {
	finished := make(chan struct{})

	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case <-t.stop:
		t.logger.WarnContext(ctx, "прогон синхронизации прерван остановкой сервиса")
	case <-ctx.Done():
		t.logger.WarnContext(ctx, "прогон синхронизации прерван отменой контекста")
	}
}
