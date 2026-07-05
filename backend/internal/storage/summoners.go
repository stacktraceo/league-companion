package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
)

// Summoners — доступ к таблице отслеживаемых саммонеров.
type Summoners struct {
	pool *pgxpool.Pool
}

func NewSummoners(pool *pgxpool.Pool) *Summoners {
	return &Summoners{pool: pool}
}

// Upsert создаёт или обновляет запись саммонера и сообщает, была ли она создана.
//
// Здесь именно DO UPDATE, а не DO NOTHING из CLAUDE.md (отклонение 2): то правило
// про matches и match_participants, где повторная вставка означает гонку двух
// синхронизаций. Повторное же добавление саммонера обязано подтянуть свежие уровень
// и иконку, иначе профиль замрёт на значениях первого дня.
//
// created_at и last_synced_at не трогаем: первый принадлежит моменту создания,
// второй — синхронизации (MarkSynced).
func (r *Summoners) Upsert(ctx context.Context, summoner domain.Summoner) (bool, error) {
	// xmax = 0 у возвращённой строки означает, что она вставлена, а не обновлена.
	// Так хендлер отличает 201 от 200, не делая лишнего SELECT и не открывая окно
	// для гонки между проверкой и вставкой.
	const query = `
		INSERT INTO summoners (puuid, riot_id, tag_line, region, summoner_level, profile_icon_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (puuid) DO UPDATE SET
			riot_id         = EXCLUDED.riot_id,
			tag_line        = EXCLUDED.tag_line,
			region          = EXCLUDED.region,
			summoner_level  = EXCLUDED.summoner_level,
			profile_icon_id = EXCLUDED.profile_icon_id
		RETURNING xmax = 0`

	var created bool

	err := r.pool.QueryRow(ctx, query,
		summoner.PUUID, summoner.RiotID, summoner.TagLine, summoner.Region,
		summoner.SummonerLevel, summoner.ProfileIconID).Scan(&created)
	if err != nil {
		return false, fmt.Errorf("storage: сохранение саммонера: %w", err)
	}

	return created, nil
}

// ByPUUID возвращает саммонера или ErrNotFound.
func (r *Summoners) ByPUUID(ctx context.Context, puuid string) (domain.Summoner, error) {
	const query = `
		SELECT puuid, riot_id, tag_line, region, summoner_level, profile_icon_id,
		       last_synced_at, created_at
		FROM summoners
		WHERE puuid = $1`

	summoner, err := scanSummoner(r.pool.QueryRow(ctx, query, puuid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Summoner{}, fmt.Errorf("%w: саммонер %s", ErrNotFound, puuid)
		}

		return domain.Summoner{}, fmt.Errorf("storage: чтение саммонера: %w", err)
	}

	return summoner, nil
}

// All возвращает всех отслеживаемых саммонеров — по этому списку ходит фоновая
// синхронизация (SPEC.md 3.5).
func (r *Summoners) All(ctx context.Context) ([]domain.Summoner, error) {
	const query = `
		SELECT puuid, riot_id, tag_line, region, summoner_level, profile_icon_id,
		       last_synced_at, created_at
		FROM summoners
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("storage: чтение списка саммонеров: %w", err)
	}
	defer rows.Close()

	var summoners []domain.Summoner

	for rows.Next() {
		summoner, err := scanSummoner(rows)
		if err != nil {
			return nil, fmt.Errorf("storage: разбор саммонера: %w", err)
		}

		summoners = append(summoners, summoner)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: чтение списка саммонеров: %w", err)
	}

	return summoners, nil
}

// TrackedPUUIDs — множество отслеживаемых puuid.
//
// Нужно синхронизации: на match_participants стоит FK на summoners, поэтому строки
// заводятся только для тех, кого мы трекаем (CLAUDE.md, отклонение 1). Заодно это
// покрывает случай, когда в одном матче встретились двое отслеживаемых.
func (r *Summoners) TrackedPUUIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `SELECT puuid FROM summoners`)
	if err != nil {
		return nil, fmt.Errorf("storage: чтение отслеживаемых puuid: %w", err)
	}
	defer rows.Close()

	tracked := make(map[string]struct{})

	for rows.Next() {
		var puuid string
		if err := rows.Scan(&puuid); err != nil {
			return nil, fmt.Errorf("storage: разбор puuid: %w", err)
		}

		tracked[puuid] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: чтение отслеживаемых puuid: %w", err)
	}

	return tracked, nil
}

// MarkSynced отмечает момент успешной синхронизации.
func (r *Summoners) MarkSynced(ctx context.Context, puuid string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE summoners SET last_synced_at = $2 WHERE puuid = $1`, puuid, at)
	if err != nil {
		return fmt.Errorf("storage: отметка синхронизации: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: саммонер %s", ErrNotFound, puuid)
	}

	return nil
}

// scanner — общий интерфейс pgx.Row и pgx.Rows в части Scan.
type scanner interface {
	Scan(dest ...any) error
}

func scanSummoner(row scanner) (domain.Summoner, error) {
	var (
		summoner      domain.Summoner
		summonerLevel *int
		profileIconID *int
	)

	err := row.Scan(
		&summoner.PUUID,
		&summoner.RiotID,
		&summoner.TagLine,
		&summoner.Region,
		&summonerLevel,
		&profileIconID,
		&summoner.LastSyncedAt,
		&summoner.CreatedAt,
	)
	if err != nil {
		return domain.Summoner{}, err
	}

	// Колонки объявлены nullable (SPEC.md 3.3): профиль мог не успеть подтянуться.
	if summonerLevel != nil {
		summoner.SummonerLevel = *summonerLevel
	}

	if profileIconID != nil {
		summoner.ProfileIconID = *profileIconID
	}

	return summoner, nil
}
