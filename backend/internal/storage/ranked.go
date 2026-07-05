package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
)

// RankedStats — доступ к ранговым снапшотам.
type RankedStats struct {
	pool *pgxpool.Pool
}

func NewRankedStats(pool *pgxpool.Pool) *RankedStats {
	return &RankedStats{pool: pool}
}

// Replace заменяет ранговый снапшот саммонера целиком.
//
// Именно замена, а не слияние: League-V4 отдаёт полный список очередей, и если
// игрок перестал играть флекс, старую запись надо убрать, а не оставить висеть
// с прошлогодним рангом. Пустой список — валидный случай: саммонер без ранга.
func (r *RankedStats) Replace(ctx context.Context, puuid string, stats []domain.RankedStat) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM ranked_stats WHERE puuid = $1`, puuid); err != nil {
			return fmt.Errorf("очистка рангов: %w", err)
		}

		const insert = `
			INSERT INTO ranked_stats
				(puuid, queue_type, tier, rank, league_points, wins, losses, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

		for _, stat := range stats {
			_, err := tx.Exec(ctx, insert,
				puuid, stat.QueueType, stat.Tier, stat.Rank,
				stat.LeaguePoints, stat.Wins, stat.Losses, stat.UpdatedAt)
			if err != nil {
				return fmt.Errorf("вставка ранга %s: %w", stat.QueueType, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("storage: сохранение рангов: %w", err)
	}

	return nil
}

// ByPUUID возвращает ранги саммонера по всем очередям. Пустой результат —
// не ошибка: саммонер может быть без ранга.
func (r *RankedStats) ByPUUID(ctx context.Context, puuid string) ([]domain.RankedStat, error) {
	const query = `
		SELECT puuid, queue_type, tier, rank, league_points, wins, losses, updated_at
		FROM ranked_stats
		WHERE puuid = $1
		ORDER BY queue_type`

	rows, err := r.pool.Query(ctx, query, puuid)
	if err != nil {
		return nil, fmt.Errorf("storage: чтение рангов: %w", err)
	}
	defer rows.Close()

	var stats []domain.RankedStat

	for rows.Next() {
		var (
			stat         domain.RankedStat
			tier         *string
			rank         *string
			leaguePoints *int
			wins         *int
			losses       *int
		)

		err := rows.Scan(&stat.PUUID, &stat.QueueType, &tier, &rank,
			&leaguePoints, &wins, &losses, &stat.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("storage: разбор ранга: %w", err)
		}

		stat.Tier = derefOr(tier, "")
		stat.Rank = derefOr(rank, "")
		stat.LeaguePoints = derefOr(leaguePoints, 0)
		stat.Wins = derefOr(wins, 0)
		stat.Losses = derefOr(losses, 0)

		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: чтение рангов: %w", err)
	}

	return stats, nil
}

// derefOr разыменовывает указатель из nullable-колонки.
func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}

	return *value
}
