package cache

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	cache   Cache
	advance func(d time.Duration)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newMemoryFixture(t *testing.T) fixture {
	t.Helper()

	now := time.Now()
	memory := NewMemory()
	memory.now = func() time.Time { return now }

	return fixture{
		cache:   memory,
		advance: func(d time.Duration) { now = now.Add(d) },
	}
}

func newRedisFixture(t *testing.T) fixture {
	t.Helper()

	server := miniredis.RunT(t)
	cache := NewRedis(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	t.Cleanup(func() { _ = cache.Close() })

	return fixture{
		cache:   cache,
		advance: server.FastForward,
	}
}

func TestCacheImplementations(t *testing.T) {
	implementations := map[string]func(*testing.T) fixture{
		"memory": newMemoryFixture,
		"redis":  newRedisFixture,
	}

	for name, newFixture := range implementations {
		t.Run(name, func(t *testing.T) {
			runCacheContract(t, newFixture)
		})
	}
}

func runCacheContract(t *testing.T, newFixture func(*testing.T) fixture) {
	t.Helper()

	ctx := context.Background()

	t.Run("неизвестный ключ - промах без ошибки", func(t *testing.T) {
		f := newFixture(t)

		value, ok, err := f.cache.Get(ctx, "нет-такого")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, value)
	})

	t.Run("значение читается обратно", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte(`{"puuid":"abc"}`), time.Minute))

		value, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte(`{"puuid":"abc"}`), value)
	})

	t.Run("повторный Set перезаписывает значение", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("старое"), time.Minute))
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("новое"), time.Minute))

		value, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte("новое"), value)
	})

	// League-V4 у игрока без ранга отдаёт пустой массив - это валидный ответ,
	// и кэшировать его надо, иначе такой игрок каждый раз ходит в Riot.
	t.Run("пустое значение - попадание, а не промах", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte(`[]`), time.Minute))

		value, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte(`[]`), value)
	})

	t.Run("значение живо почти весь TTL", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("значение"), time.Minute))

		f.advance(59 * time.Second)

		_, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("после TTL - промах", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("значение"), time.Minute))

		f.advance(time.Minute + time.Second)

		_, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("нулевой TTL ничего не сохраняет", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("значение"), 0))

		_, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Get отдаёт независимую копию", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.cache.Set(ctx, "ключ", []byte("значение"), time.Minute))

		value, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)

		value[0] = 'X'

		again, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte("значение"), again)
	})

	t.Run("Set копирует переданный срез", func(t *testing.T) {
		f := newFixture(t)

		value := []byte("значение")
		require.NoError(t, f.cache.Set(ctx, "ключ", value, time.Minute))
		value[0] = 'X'

		stored, ok, err := f.cache.Get(ctx, "ключ")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte("значение"), stored)
	})
}

func TestMemorySweepsExpiredEntries(t *testing.T) {
	now := time.Now()
	memory := NewMemory()
	memory.now = func() time.Time { return now }

	ctx := context.Background()

	for i := range minSweepSize * 2 {
		require.NoError(t, memory.Set(ctx, string(rune('a'+i%26))+string(rune('a'+i/26)), []byte("v"), time.Minute))
	}

	require.NotEmpty(t, memory.items)

	now = now.Add(2 * time.Minute)

	// Один Set после протухания всех ключей должен вычистить накопленное,
	// а не оставить его лежать до перезапуска.
	require.NoError(t, memory.Set(ctx, "свежий", []byte("v"), time.Minute))

	assert.Len(t, memory.items, 1)
}

func TestMemoryIsSafeForConcurrentUse(t *testing.T) {
	memory := NewMemory()
	ctx := context.Background()

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			assert.NoError(t, memory.Set(ctx, "ключ", []byte{byte(i)}, time.Minute))
		}()

		go func() {
			defer wg.Done()

			_, _, err := memory.Get(ctx, "ключ")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestRedisLoggerWritesToSlog(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	redisLogger{logger: logger}.Printf(context.Background(), "failed to dial after %d attempts", 5)

	assert.Contains(t, buf.String(), "failed to dial after 5 attempts")
	assert.Contains(t, buf.String(), "source=go-redis")
	assert.Contains(t, buf.String(), "level=WARN")
}

func TestOpenFallsBackToMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("пустой адрес", func(t *testing.T) {
		c := Open(ctx, "", discardLogger())
		t.Cleanup(func() { _ = c.Close() })

		assert.IsType(t, &Memory{}, c)
	})

	t.Run("redis недоступен", func(t *testing.T) {
		// Порт 1 гарантированно никем не слушается.
		c := Open(ctx, "127.0.0.1:1", discardLogger())
		t.Cleanup(func() { _ = c.Close() })

		assert.IsType(t, &Memory{}, c)
	})
}

func TestOpenUsesRedisWhenAvailable(t *testing.T) {
	server := miniredis.RunT(t)

	c := Open(context.Background(), server.Addr(), discardLogger())
	t.Cleanup(func() { _ = c.Close() })

	require.IsType(t, &Redis{}, c)

	require.NoError(t, c.Set(context.Background(), "ключ", []byte("значение"), time.Minute))

	// Префикс отделяет ключи сервиса от чужих данных в том же инстансе.
	assert.True(t, server.Exists(keyPrefix+"ключ"))
}
