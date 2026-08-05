package syncer

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// Параметры фонового исполнителя.
const (
	// DefaultWorkers — сколько саммонеров синхронизируются одновременно (SPEC.md 3.5).
	// Больше смысла нет: все воркеры всё равно стоят в очереди к общему лимитеру Riot.
	DefaultWorkers = 3

	// DefaultQueueSize — глубина очереди. Очередь нужна только чтобы всплеск
	// добавлений не отбрасывался; накапливать в ней тысячи задач незачем.
	DefaultQueueSize = 64

	// jobTimeout — потолок на одну синхронизацию. Двадцать матчей под лимитом
	// Riot укладываются в минуты, но висеть вечно задача не должна.
	jobTimeout = 10 * time.Minute
)

// Syncer — то, что умеет синхронизировать одного саммонера.
type Syncer interface {
	SyncSummoner(ctx context.Context, puuid string, count int) (int, error)
}

type job struct {
	puuid string
	count int

	// done вызывается после завершения задачи — так тикер узнаёт, что его пачка
	// доработана. У задач от HTTP-хендлеров nil: им ждать нечего.
	done func()
}

// Runner выполняет синхронизации в фоне.
//
// В вехе 7 поверх него встанет тикер с guard'ом от наложения прогонов
// (DECISIONS.md, отклонение 4); сама очередь и воркеры останутся теми же.
type Runner struct {
	service Syncer
	logger  *slog.Logger

	jobs chan job
	quit chan struct{}

	workers int

	wg       sync.WaitGroup
	stopOnce sync.Once
	cancel   context.CancelFunc
}

// NewRunner создаёт исполнителя. workers и queueSize меньше единицы поднимаются до неё.
func NewRunner(service Syncer, workers, queueSize int, logger *slog.Logger) *Runner {
	workers = max(workers, 1)
	queueSize = max(queueSize, 1)

	return &Runner{
		service: service,
		logger:  logger,
		jobs:    make(chan job, queueSize),
		quit:    make(chan struct{}),
		workers: workers,
	}
}

// Start запускает воркеров.
//
// base становится родителем контекстов задач, поэтому это должен быть контекст
// приложения, а не HTTP-запроса: запрос завершается сразу после ответа, а
// синхронизация продолжается.
func (r *Runner) Start(base context.Context) {
	ctx, cancel := context.WithCancel(base)
	r.cancel = cancel

	for range r.workers {
		r.wg.Add(1)

		go r.worker(ctx)
	}

	r.logger.InfoContext(ctx, "фоновая синхронизация запущена",
		"воркеров", r.workers, "очередь", cap(r.jobs))
}

// Enqueue ставит саммонера в очередь на синхронизацию.
//
// Никогда не блокирует вызывающего: HTTP-хендлер не должен ждать фон. Если очередь
// переполнена, задача отбрасывается — саммонер уже сохранён, и его подберёт
// следующий прогон.
func (r *Runner) Enqueue(puuid string, count int) bool {
	return r.Submit(puuid, count, nil)
}

// Submit — то же, что Enqueue, но с колбэком завершения. Нужен тикеру, который
// дожидается своей пачки, прежде чем снять guard от наложения прогонов.
//
// Если задача не принята, done не вызывается — вызывающий обязан обработать это сам
// по возвращённому false. Иначе счётчик ожидания разошёлся бы с реальностью.
func (r *Runner) Submit(puuid string, count int, done func()) bool {
	select {
	case <-r.quit:
		return false
	default:
	}

	select {
	case r.jobs <- job{puuid: puuid, count: count, done: done}:
		return true
	default:
		r.logger.Warn("очередь синхронизации переполнена, задача отброшена", "puuid", puuid)

		return false
	}
}

// Shutdown прекращает приём задач и дожидается активных.
//
// Задачи, оставшиеся в очереди, не выполняются: каждая — это десятки запросов
// к Riot, и растягивать из-за них остановку сервиса неправильно. Если дождаться
// активных не удалось до истечения ctx, они отменяются принудительно.
func (r *Runner) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.quit) })

	done := make(chan struct{})

	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if r.cancel != nil {
			r.cancel()
		}

		// Горутины всё равно дожидаемся: иначе они переживут возврат из Shutdown.
		<-done

		return ctx.Err()
	}
}

func (r *Runner) worker(ctx context.Context) {
	defer r.wg.Done()

	for {
		// Отдельная проверка перед основным select: тот выбирает готовый case
		// случайно и после закрытия quit мог бы взять ещё одну задачу.
		select {
		case <-r.quit:
			return
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-r.quit:
			return
		case <-ctx.Done():
			return
		case next := <-r.jobs:
			r.run(ctx, next)
		}
	}
}

func (r *Runner) run(ctx context.Context, next job) {
	// done обязан сработать при любом исходе, включая панику: иначе тикер навсегда
	// останется в состоянии «прогон идёт» и больше не запустится.
	if next.done != nil {
		defer next.done()
	}

	// Паника в фоновой горутине роняет процесс целиком: Recoverer из httpapi
	// сюда не достаёт.
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logger.ErrorContext(ctx, "паника в фоновой синхронизации",
				"puuid", next.puuid,
				"panic", recovered,
				"stack", string(debug.Stack()))
		}
	}()

	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	started := time.Now()

	synced, err := r.service.SyncSummoner(jobCtx, next.puuid, next.count)
	if err != nil {
		r.logger.ErrorContext(jobCtx, "фоновая синхронизация не удалась",
			"puuid", next.puuid, "error", err, "duration", time.Since(started))

		return
	}

	r.logger.InfoContext(jobCtx, "фоновая синхронизация завершена",
		"puuid", next.puuid, "новых матчей", synced, "duration", time.Since(started))
}
