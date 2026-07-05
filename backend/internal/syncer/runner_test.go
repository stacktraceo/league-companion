package syncer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitFor ждёт условие, не привязываясь к конкретной паузе: тесты на конкурентность
// иначе флакают под нагрузкой.
func waitFor(t *testing.T, timeout time.Duration, message string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("не дождались: %s", message)
}

// fakeSyncer позволяет держать задачу «в работе» столько, сколько нужно тесту.
type fakeSyncer struct {
	mu   sync.Mutex
	seen []string

	started chan string   // сигнал «задача началась»
	release chan struct{} // задача ждёт закрытия этого канала
	err     error
	panics  bool

	blockUntilCtxDone bool
}

func (f *fakeSyncer) SyncSummoner(ctx context.Context, puuid string, _ int) (int, error) {
	f.mu.Lock()
	f.seen = append(f.seen, puuid)
	shouldPanic := f.panics
	f.mu.Unlock()

	if f.started != nil {
		f.started <- puuid
	}

	if shouldPanic {
		panic("нарочно")
	}

	if f.blockUntilCtxDone {
		<-ctx.Done()

		return 0, ctx.Err()
	}

	if f.release != nil {
		<-f.release
	}

	return 1, f.err
}

func (f *fakeSyncer) Seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.seen...)
}

func (f *fakeSyncer) setPanics(panics bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.panics = panics
}

func newTestRunner(t *testing.T, service Syncer, workers, queueSize int) *Runner {
	t.Helper()

	runner := NewRunner(service, workers, queueSize, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = runner.Shutdown(ctx)
	})

	return runner
}

func TestRunnerProcessesEnqueuedJobs(t *testing.T) {
	service := &fakeSyncer{}
	runner := newTestRunner(t, service, 2, 8)

	for _, puuid := range []string{"puuid-1", "puuid-2", "puuid-3"} {
		assert.True(t, runner.Enqueue(puuid, 10))
	}

	waitFor(t, 2*time.Second, "все три задачи выполнены", func() bool {
		return len(service.Seen()) == 3
	})

	assert.ElementsMatch(t, []string{"puuid-1", "puuid-2", "puuid-3"}, service.Seen())
}

// HTTP-хендлер не должен ждать фон: при переполненной очереди Enqueue обязан
// вернуться сразу и сказать «не взял».
func TestEnqueueNeverBlocks(t *testing.T) {
	service := &fakeSyncer{
		started: make(chan string, 1),
		release: make(chan struct{}),
	}
	defer close(service.release)

	runner := newTestRunner(t, service, 1, 1)

	require.True(t, runner.Enqueue("занимает-воркер", 10))

	select {
	case <-service.started:
	case <-time.After(2 * time.Second):
		t.Fatal("воркер не взял задачу")
	}

	require.True(t, runner.Enqueue("занимает-очередь", 10))

	// Очередь на 1 забита, единственный воркер занят — эта задача должна отвалиться.
	done := make(chan bool, 1)

	go func() { done <- runner.Enqueue("лишняя", 10) }()

	select {
	case accepted := <-done:
		assert.False(t, accepted, "переполненная очередь должна отбрасывать задачу")
	case <-time.After(time.Second):
		t.Fatal("Enqueue заблокировался на переполненной очереди")
	}
}

func TestShutdownWaitsForRunningJob(t *testing.T) {
	service := &fakeSyncer{
		started: make(chan string, 1),
		release: make(chan struct{}),
	}

	runner := NewRunner(service, 1, 4, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	require.True(t, runner.Enqueue("puuid-1", 10))
	<-service.started

	finished := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		finished <- runner.Shutdown(ctx)
	}()

	select {
	case <-finished:
		t.Fatal("Shutdown вернулся, не дождавшись активной задачи")
	case <-time.After(50 * time.Millisecond):
	}

	close(service.release)

	select {
	case err := <-finished:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown не завершился после окончания задачи")
	}
}

// Оставшиеся в очереди задачи не выполняются: каждая — это десятки запросов
// к Riot, растягивать из-за них остановку сервиса неправильно.
func TestShutdownDropsQueuedJobs(t *testing.T) {
	service := &fakeSyncer{
		started: make(chan string, 1),
		release: make(chan struct{}),
	}

	runner := NewRunner(service, 1, 8, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	require.True(t, runner.Enqueue("активная", 10))
	<-service.started

	for range 3 {
		require.True(t, runner.Enqueue("в-очереди", 10))
	}

	close(service.release)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, runner.Shutdown(ctx))

	assert.Equal(t, []string{"активная"}, service.Seen())
}

func TestShutdownCancelsJobWhenDeadlinePasses(t *testing.T) {
	service := &fakeSyncer{
		started:           make(chan string, 1),
		blockUntilCtxDone: true,
	}

	runner := NewRunner(service, 1, 4, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	require.True(t, runner.Enqueue("puuid-1", 10))
	<-service.started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := runner.Shutdown(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded,
		"не дождавшись задачи, Shutdown обязан сказать об этом")
}

func TestEnqueueAfterShutdownIsRejected(t *testing.T) {
	runner := NewRunner(&fakeSyncer{}, 1, 4, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, runner.Shutdown(ctx))

	assert.False(t, runner.Enqueue("puuid-1", 10), "после остановки задачу брать некому")
}

func TestShutdownIsIdempotent(t *testing.T) {
	runner := NewRunner(&fakeSyncer{}, 1, 4, slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Start(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, runner.Shutdown(ctx))
	require.NoError(t, runner.Shutdown(ctx), "повторный вызов не должен паниковать на закрытом канале")
}

// Паника в фоновой горутине роняет процесс целиком — Recoverer из httpapi сюда
// не достаёт, поэтому воркер обязан ловить её сам.
func TestPanicInJobDoesNotKillWorker(t *testing.T) {
	service := &fakeSyncer{panics: true}
	runner := newTestRunner(t, service, 1, 4)

	require.True(t, runner.Enqueue("паникующая", 10))

	waitFor(t, 2*time.Second, "первая задача обработана", func() bool {
		return len(service.Seen()) == 1
	})

	service.setPanics(false)
	require.True(t, runner.Enqueue("следующая", 10))

	waitFor(t, 2*time.Second, "воркер жив и взял следующую задачу", func() bool {
		return len(service.Seen()) == 2
	})
}

func TestJobErrorDoesNotStopRunner(t *testing.T) {
	service := &fakeSyncer{err: errors.New("riot прилёг")}
	runner := newTestRunner(t, service, 1, 4)

	require.True(t, runner.Enqueue("puuid-1", 10))
	require.True(t, runner.Enqueue("puuid-2", 10))

	waitFor(t, 2*time.Second, "обе задачи обработаны несмотря на ошибки", func() bool {
		return len(service.Seen()) == 2
	})
}
