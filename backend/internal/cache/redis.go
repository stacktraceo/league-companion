package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// keyPrefix отделяет ключи сервиса от всего остального, что может жить
// в том же инстансе Redis.
const keyPrefix = "lc:"

// Redis — кэш поверх Redis. В отличие от Memory переживает рестарт сервиса
// и общий для нескольких инстансов.
type Redis struct {
	client *redis.Client
}

// NewRedis оборачивает готовый клиент. Владение клиентом переходит к кэшу:
// Close закрывает именно его.
func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := r.client.Get(ctx, keyPrefix+key).Bytes()

	switch {
	case errors.Is(err, redis.Nil):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("cache: чтение %q из redis: %w", key, err)
	}

	return value, true, nil
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Без TTL ключи Riot оседали бы в Redis навсегда — сюда попадают только
	// данные, которые обязаны протухать (SPEC.md 3.1).
	if ttl <= 0 {
		return nil
	}

	if err := r.client.Set(ctx, keyPrefix+key, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache: запись %q в redis: %w", key, err)
	}

	return nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}
