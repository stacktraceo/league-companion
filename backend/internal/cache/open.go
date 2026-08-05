package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// pingTimeout - сколько ждём Redis на старте. Дольше держать запуск сервиса
	// смысла нет: кэш не обязателен.
	pingTimeout = 2 * time.Second

	// Таймауты на операции с кэшем нарочно жёсткие: кэш существует, чтобы
	// экономить поход в Riot, и не должен обходиться дороже него.
	dialTimeout = 2 * time.Second
	opTimeout   = time.Second

	// maxRetries - одна повторная попытка. Дальше дешевле сходить в Riot.
	maxRetries = 1
)

type redisLogger struct {
	logger *slog.Logger
}

func (l redisLogger) Printf(ctx context.Context, format string, v ...any) {
	l.logger.WarnContext(ctx, fmt.Sprintf(format, v...), "source", "go-redis")
}

func Open(ctx context.Context, addr string, logger *slog.Logger) Cache {
	if addr == "" {
		logger.Info("REDIS_ADDR не задан, кэш работает в памяти процесса")

		return NewMemory()
	}

	// SetLogger глобален - другой точки перехвата go-redis не предлагает.
	redis.SetLogger(redisLogger{logger: logger})

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  dialTimeout,
		ReadTimeout:  opTimeout,
		WriteTimeout: opTimeout,
		MaxRetries:   maxRetries,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("redis недоступен, переключаюсь на кэш в памяти процесса",
			"addr", addr, "error", err)

		_ = client.Close()

		return NewMemory()
	}

	logger.Info("кэш работает через redis", "addr", addr)

	return NewRedis(client)
}
