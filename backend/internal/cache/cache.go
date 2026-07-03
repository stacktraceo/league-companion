// Package cache — кэш ответов Riot API с TTL (SPEC.md 3.1).
//
// Значение — сырой JSON ответа, ключ строит вызывающий код. Кэш здесь всегда
// необязателен: промах или ошибка означают лишь поход в Riot, но не отказ запроса.
package cache

import (
	"context"
	"time"
)

// Cache — хранилище байтов с временем жизни.
//
// Get возвращает ok=false на промахе; ошибка — только про сбой самого хранилища.
// Пустое значение при этом остаётся полноценным попаданием: у Riot есть эндпоинты,
// легитимно отдающие пустой массив (например, League-V4 у игрока без ранга).
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Close() error
}
