package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
)

// MatchListItem — строка ленты матчей: сам матч плюс участие саммонера в нём.
//
// Сырого JSON здесь намеренно нет: он весит сотни килобайт на матч и нужен только
// в GET /api/v1/matches/{matchId}.
type MatchListItem struct {
	MatchID      string
	GameCreation time.Time
	GameDuration time.Duration
	QueueID      int
	GameVersion  string

	ChampionName string
	Kills        int
	Deaths       int
	Assists      int
	Win          bool
	CS           int
	GoldEarned   int
}

// Matches — доступ к матчам и участию в них.
type Matches struct {
	pool *pgxpool.Pool
}

func NewMatches(pool *pgxpool.Pool) *Matches {
	return &Matches{pool: pool}
}

// KnownIDs возвращает подмножество ids, которое уже лежит в БД.
//
// Синхронизация спрашивает это до похода в Riot: детали матча весят ~140 КБ, и
// перекачивать уже сохранённые — впустую жечь лимит ключа.
func (r *Matches) KnownIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	known := make(map[string]struct{}, len(ids))

	if len(ids) == 0 {
		return known, nil
	}

	rows, err := r.pool.Query(ctx, `SELECT match_id FROM matches WHERE match_id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("storage: проверка известных матчей: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("storage: разбор match_id: %w", err)
		}

		known[id] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: проверка известных матчей: %w", err)
	}

	return known, nil
}

// Insert сохраняет матч и участие отслеживаемых саммонеров.
//
// Обе вставки идут через ON CONFLICT DO NOTHING (CLAUDE.md, отклонение 2):
// один матч может прийти сразу из нескольких параллельных синхронизаций, если в нём
// встретились двое отслеживаемых. Транзакция обязательна — match_participants
// ссылается на matches внешним ключом.
func (r *Matches) Insert(
	ctx context.Context,
	match domain.Match,
	participants []domain.MatchParticipant,
) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		const insertMatch = `
			INSERT INTO matches (match_id, game_creation, game_duration, queue_id, game_version, raw_data)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (match_id) DO NOTHING`

		_, err := tx.Exec(ctx, insertMatch,
			match.MatchID, match.GameCreation, match.DurationSeconds(),
			match.QueueID, match.GameVersion, []byte(match.RawData))
		if err != nil {
			return fmt.Errorf("вставка матча: %w", err)
		}

		const insertParticipant = `
			INSERT INTO match_participants
				(match_id, puuid, champion_name, kills, deaths, assists, win, cs, gold_earned)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (match_id, puuid) DO NOTHING`

		for _, p := range participants {
			_, err := tx.Exec(ctx, insertParticipant,
				p.MatchID, p.PUUID, p.ChampionName, p.Kills, p.Deaths,
				p.Assists, p.Win, p.CS, p.GoldEarned)
			if err != nil {
				return fmt.Errorf("вставка участия %s: %w", p.PUUID, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("storage: сохранение матча %s: %w", match.MatchID, err)
	}

	return nil
}

// ListByPUUID возвращает ленту матчей саммонера, свежие первыми.
func (r *Matches) ListByPUUID(ctx context.Context, puuid string, limit, offset int) ([]MatchListItem, error) {
	const query = `
		SELECT m.match_id, m.game_creation, m.game_duration, m.queue_id, m.game_version,
		       p.champion_name, p.kills, p.deaths, p.assists, p.win, p.cs, p.gold_earned
		FROM match_participants p
		JOIN matches m ON m.match_id = p.match_id
		WHERE p.puuid = $1
		ORDER BY m.game_creation DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, puuid, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("storage: чтение ленты матчей: %w", err)
	}
	defer rows.Close()

	items := make([]MatchListItem, 0, limit)

	for rows.Next() {
		var (
			item            MatchListItem
			durationSeconds int
		)

		err := rows.Scan(
			&item.MatchID, &item.GameCreation, &durationSeconds, &item.QueueID, &item.GameVersion,
			&item.ChampionName, &item.Kills, &item.Deaths, &item.Assists,
			&item.Win, &item.CS, &item.GoldEarned,
		)
		if err != nil {
			return nil, fmt.Errorf("storage: разбор строки ленты: %w", err)
		}

		item.GameDuration = time.Duration(durationSeconds) * time.Second
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: чтение ленты матчей: %w", err)
	}

	return items, nil
}

// CountByPUUID — сколько всего матчей сохранено у саммонера. Нужно клиенту,
// чтобы понимать границы пагинации.
func (r *Matches) CountByPUUID(ctx context.Context, puuid string) (int, error) {
	var total int

	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM match_participants WHERE puuid = $1`, puuid).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("storage: подсчёт матчей: %w", err)
	}

	return total, nil
}

// RawByID отдаёт исходный JSON Match-V5 — из него собираются полные детали матча
// со всеми десятью участниками (CLAUDE.md, отклонение 1).
func (r *Matches) RawByID(ctx context.Context, matchID string) (json.RawMessage, error) {
	var raw []byte

	err := r.pool.QueryRow(ctx, `SELECT raw_data FROM matches WHERE match_id = $1`, matchID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: матч %s", ErrNotFound, matchID)
		}

		return nil, fmt.Errorf("storage: чтение матча: %w", err)
	}

	return raw, nil
}
