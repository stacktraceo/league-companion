package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// pingTimeout — сколько ждём Redis на старте. Дольше держать запуск сервиса
// смысла нет: кэш не обязателен.
const pingTimeout = 2 * time.Second

// Open подключается к Redis по адресу addr.
//
// Пустой адрес или недоступный Redis — не ошибка: сервис продолжает работать
// на кэше в памяти, о чём честно пишет в лог. Кэш ускоряет ответы и бережёт лимит
// Riot, но не является источником истины — им остаётся PostgreSQL.
func Open(ctx context.Context, addr string, logger *slog.Logger) Cache {
	if addr == "" {
		logger.Info("REDIS_ADDR не задан, кэш работает в памяти процесса")

		return NewMemory()
	}

	client := redis.NewClient(&redis.Options{Addr: addr})

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
