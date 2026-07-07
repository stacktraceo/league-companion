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

// fakeTracked отдаёт заготовленный список отслеживаемых саммонеров.
type fakeTracked struct {
	puuids []string
	err    error

	mu    sync.Mutex
	calls int
}

func (f *fakeTracked) TrackedPUUIDs(context.Context) (map[string]struct{}, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	tracked := make(map[string]struct{}, len(f.puuids))
	for _, puuid := range f.puuids {
		tracked[puuid] = struct{}{}
	}

	return tracked, nil
}

func (f *fakeTracked) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

// fakeBatchQueue считает принятые задачи и умеет задерживать их завершение —
// так проверяется guard от наложения прогонов.
type fakeBatchQueue struct {
	mu       sync.Mutex
	accepted []string
	rejected int

	reject bool

	// hold, если не nil, держит done до закрытия канала.
	hold chan struct{}
}

func (q *fakeBatchQueue) Submit(puuid string, _ int, done func()) bool {
	q.mu.Lock()

	if q.reject {
		q.rejected++
		q.mu.Unlock()

		return false
	}

	q.accepted = append(q.accepted, puuid)
	hold := q.hold
	q.mu.Unlock()

	go func() {
		if hold != nil {
			<-hold
		}

		if done != nil {
			done()
		}
	}()

	return true
}

func (q *fakeBatchQueue) Accepted() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	return append([]string(nil), q.accepted...)
}

func (q *fakeBatchQueue) Rejected() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.rejected
}

func newTestTicker(tracked TrackedStore, queue BatchQueue, interval time.Duration) *Ticker {
	return NewTicker(tracked, queue, interval, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestTickerRunOnceQueuesEveryTrackedSummoner(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1", "puuid-2", "puuid-3"}}
	queue := &fakeBatchQueue{}

	queued := newTestTicker(tracked, queue, time.Minute).runOnce(context.Background())

	assert.Equal(t, 3, queued)
	assert.ElementsMatch(t, tracked.puuids, queue.Accepted())
}

// Guard от наложения (CLAUDE.md, отклонение 4): пока предыдущий прогон не завершён,
// следующий тик пропускается, а не встаёт в очередь.
func TestTickerSkipsTickWhileRunning(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1"}}
	hold := make(chan struct{})
	queue := &fakeBatchQueue{hold: hold}
	ticker := newTestTicker(tracked, queue, time.Minute)

	// Первый прогон повиснет на незавершённой задаче.
	firstDone := make(chan int, 1)

	go func() { firstDone <- ticker.runOnce(context.Background()) }()

	// Ждём, пока первый прогон действительно займёт guard.
	require.Eventually(t, func() bool { return len(queue.Accepted()) == 1 },
		time.Second, time.Millisecond, "первый прогон должен успеть поставить задачу")

	assert.Zero(t, ticker.runOnce(context.Background()), "второй тик обязан быть пропущен")
	assert.Len(t, queue.Accepted(), 1, "пропущенный тик не ставит задачи повторно")

	close(hold)
	assert.Equal(t, 1, <-firstDone)

	// Guard снят — следующий прогон снова работает.
	assert.Equal(t, 1, ticker.runOnce(context.Background()))
	assert.Len(t, queue.Accepted(), 2)
}

// Пропущенный тик не копится: после освобождения guard'а выполняется один прогон,
// а не столько, сколько тиков пропустили.
func TestTickerSkippedTicksDoNotQueueUp(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1"}}
	hold := make(chan struct{})
	queue := &fakeBatchQueue{hold: hold}
	ticker := newTestTicker(tracked, queue, time.Minute)

	go ticker.runOnce(context.Background())
	require.Eventually(t, func() bool { return len(queue.Accepted()) == 1 },
		time.Second, time.Millisecond)

	for range 5 {
		assert.Zero(t, ticker.runOnce(context.Background()))
	}

	close(hold)

	require.Eventually(t, func() bool { return !ticker.running.Load() },
		time.Second, time.Millisecond)
	assert.Len(t, queue.Accepted(), 1, "пять пропущенных тиков не превратились в пять прогонов")
}

// Переполненная очередь не должна оставить guard навсегда занятым: у отброшенной
// задачи никто не позовёт done, и счётчик ожидания снимает сам тикер.
func TestTickerSurvivesFullQueue(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1", "puuid-2"}}
	queue := &fakeBatchQueue{reject: true}
	ticker := newTestTicker(tracked, queue, time.Minute)

	done := make(chan int, 1)
	go func() { done <- ticker.runOnce(context.Background()) }()

	select {
	case queued := <-done:
		assert.Zero(t, queued)
	case <-time.After(time.Second):
		t.Fatal("прогон завис на ожидании отброшенных задач")
	}

	assert.Equal(t, 2, queue.Rejected())
	assert.False(t, ticker.running.Load(), "guard снят")
}

func TestTickerWithoutTrackedSummoners(t *testing.T) {
	queue := &fakeBatchQueue{}

	assert.Zero(t, newTestTicker(&fakeTracked{}, queue, time.Minute).runOnce(context.Background()))
	assert.Empty(t, queue.Accepted())
}

// Недоступная база не должна ронять тикер: следующий тик попробует снова.
func TestTickerSurvivesStoreFailure(t *testing.T) {
	tracked := &fakeTracked{err: errors.New("база прилегла")}
	ticker := newTestTicker(tracked, &fakeBatchQueue{}, time.Minute)

	assert.Zero(t, ticker.runOnce(context.Background()))
	assert.False(t, ticker.running.Load(), "guard снят даже при ошибке")
}

// Тикер действительно тикает: с коротким интервалом прогоны идут сами.
func TestTickerFiresOnInterval(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1"}}
	queue := &fakeBatchQueue{}
	ticker := newTestTicker(tracked, queue, 5*time.Millisecond)

	ticker.Start(context.Background())
	t.Cleanup(ticker.Stop)

	require.Eventually(t, func() bool { return len(queue.Accepted()) >= 2 },
		2*time.Second, time.Millisecond, "за два интервала должно пройти минимум два прогона")
}

// Первый прогон — по первому тику, а не на старте: иначе рестарты сервиса давали бы
// всплеск запросов к Riot.
func TestTickerDoesNotRunOnStart(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1"}}
	queue := &fakeBatchQueue{}
	ticker := newTestTicker(tracked, queue, time.Hour)

	ticker.Start(context.Background())
	t.Cleanup(ticker.Stop)

	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, queue.Accepted())
	assert.Zero(t, tracked.Calls())
}

// Stop не должен зависать на незавершённой пачке — иначе остановка сервиса
// растянулась бы до jobTimeout.
func TestTickerStopDoesNotWaitForBatch(t *testing.T) {
	tracked := &fakeTracked{puuids: []string{"puuid-1"}}
	hold := make(chan struct{})
	defer close(hold)

	queue := &fakeBatchQueue{hold: hold}
	ticker := newTestTicker(tracked, queue, 5*time.Millisecond)

	ticker.Start(context.Background())

	require.Eventually(t, func() bool { return len(queue.Accepted()) == 1 },
		2*time.Second, time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		ticker.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop завис на незавершённой пачке")
	}
}

// Stop до Start не должен блокироваться на канале, который никто не закроет.
func TestTickerStopWithoutStart(t *testing.T) {
	ticker := newTestTicker(&fakeTracked{}, &fakeBatchQueue{}, time.Minute)

	stopped := make(chan struct{})
	go func() {
		ticker.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop без Start завис")
	}
}

// Отмена контекста приложения выводит цикл, даже если Stop не звали.
func TestTickerStopsOnContextCancel(t *testing.T) {
	ticker := newTestTicker(&fakeTracked{}, &fakeBatchQueue{}, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	ticker.Start(ctx)
	cancel()

	select {
	case <-ticker.done:
	case <-time.After(2 * time.Second):
		t.Fatal("цикл не вышел по отмене контекста")
	}
}
