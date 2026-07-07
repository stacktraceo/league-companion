package syncer

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultSyncInterval — как часто просыпается фоновая синхронизация (SPEC.md 3.5).
const DefaultSyncInterval = 10 * time.Minute

// TrackedStore отдаёт список отслеживаемых саммонеров.
type TrackedStore interface {
	TrackedPUUIDs(ctx context.Context) (map[string]struct{}, error)
}

// BatchQueue — очередь, умеющая сообщать о завершении задачи.
type BatchQueue interface {
	Submit(puuid string, count int, done func()) bool
}

// Ticker периодически прогоняет синхронизацию по всем отслеживаемым саммонерам
// (SPEC.md 3.5).
//
// Своего пула воркеров не поднимает: задачи уходят в тот же Runner, что обслуживает
// HTTP-хендлеры. Второй пул означал бы дубль логики ограничения параллелизма и вдвое
// больше горутин в очереди к общему лимитеру Riot, который всё равно один на процесс.
type Ticker struct {
	interval  time.Duration
	summoners TrackedStore
	queue     BatchQueue
	logger    *slog.Logger

	// running — guard от наложения прогонов (CLAUDE.md, отклонение 4). Цикл вызывает
	// runOnce последовательно, поэтому за пропуск тика во время долгого прогона
	// отвечает dropPendingTick; этот флаг закрывает второй путь — вызов runOnce
	// откуда-то ещё, помимо цикла.
	running atomic.Bool

	started  atomic.Bool
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

// NewTicker создаёт тикер. Интервал меньше единицы времени поднимается до значения
// по умолчанию.
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

// Start запускает цикл тикера.
//
// Первый прогон — по первому тику, а не сразу: иначе цикл перезапусков сервиса
// означал бы всплеск запросов к Riot на каждый рестарт. Свежедобавленный саммонер
// и так синхронизируется при добавлении.
func (t *Ticker) Start(ctx context.Context) {
	t.started.Store(true)

	go t.loop(ctx)

	t.logger.InfoContext(ctx, "периодическая синхронизация запущена", "интервал", t.interval)
}

// Stop останавливает цикл и дожидается его выхода.
//
// Останавливать тикер нужно раньше воркеров: иначе он продолжит подкидывать им
// задачи во время остановки сервиса.
//
// Возврат быстрый: активный прогон не дожидается своей пачки, а бросает её. Задачи
// при этом не теряются — их доделывают или отменяют воркеры в Runner.Shutdown,
// а недокачанное подберёт следующий запуск сервиса.
func (t *Ticker) Stop() {
	t.stopOnce.Do(func() { close(t.stop) })

	// Без Start канал done никто не закроет — ждать было бы вечно.
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

// dropPendingTick выбрасывает тик, случившийся за время прогона.
//
// Без этого guard от наложения не работал бы там, где он и нужен: time.Ticker
// буферизует один тик, поэтому тик во время долгого прогона не пропадает, а
// дожидается конца — и сразу запускает второй прогон подряд. Это то самое
// «встаёт в очередь», которого CLAUDE.md (отклонение 4) велит не допускать;
// atomic-guard в runOnce его не ловит, потому что цикл однопоточный и вызывает
// runOnce строго последовательно.
func (t *Ticker) dropPendingTick(ctx context.Context, ticker *time.Ticker) {
	select {
	case <-ticker.C:
		t.logger.WarnContext(ctx, "предыдущий прогон синхронизации ещё шёл, тик пропущен",
			"интервал", t.interval)
	default:
	}
}

// runOnce прогоняет синхронизацию по всем отслеживаемым саммонерам и дожидается её.
//
// Возвращает число саммонеров, поставленных в очередь; 0 означает и «никого не
// отслеживаем», и «тик пропущен».
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
			// Задача не принята — Submit не позовёт done, поэтому снимаем счётчик сами.
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

// waitBatch ждёт завершения пачки, но не дольше сигнала остановки.
//
// Ждать безусловно нельзя по двум причинам. Первая: Runner.Shutdown закрывает свой
// quit, и воркеры выходят, не дочерпав очередь — done у оставшихся задач никогда не
// вызовется. Вторая: одна задача может занять до jobTimeout, и остановка сервиса
// зависла бы на десять минут.
//
// Горутина с wg.Wait() в этом случае доживает до конца процесса. Это осознанно:
// единственная альтернатива — тащить контекст в каждую задачу Runner'а, что усложнило
// бы его ради нескольких секунд перед выходом.
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
