// Package storage отвечает за подключение к PostgreSQL и применение миграций.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Параметры пула. Development-ключ Riot жёстко ограничивает поток запросов
// (20 rps), так что большой пул смысла не имеет.
const (
	maxConns          = 10
	minConns          = 1
	maxConnLifetime   = time.Hour
	maxConnIdleTime   = 30 * time.Minute
	healthCheckPeriod = time.Minute
	connectTimeout    = 5 * time.Second
)

// Connect открывает пул соединений к PostgreSQL и проверяет его пингом.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("некорректный DATABASE_URL: %w", err)
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.HealthCheckPeriod = healthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул соединений: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("база недоступна: %w", err)
	}

	return pool, nil
}
