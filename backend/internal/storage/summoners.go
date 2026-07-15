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

// NewSummoners создаёт репозиторий саммонеров поверх пула соединений.
func NewSummoners(pool *pgxpool.Pool) *Summoners {
	return &Summoners{pool: pool}
}

// Upsert создаёт или обновляет запись саммонера и возвращает её в том виде,
// в каком она теперь лежит в базе, вместе с признаком «создана впервые».
//
// Здесь именно DO UPDATE, а не DO NOTHING из CLAUDE.md (отклонение 2): то правило
// про matches и match_participants, где повторная вставка означает гонку двух
// синхронизаций. Повторное же добавление саммонера обязано подтянуть свежие уровень
// и иконку, иначе профиль замрёт на значениях первого дня.
//
// created_at и last_synced_at не трогаем: первый принадлежит моменту создания,
// второй — синхронизации (MarkSynced). Но вернуть их обязаны: у объекта, собранного
// из ответа Riot, этих полей нет, и без RETURNING наружу уходил бы нулевой
// created_at.
func (r *Summoners) Upsert(ctx context.Context, summoner domain.Summoner) (domain.Summoner, bool, error) {
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
		RETURNING puuid, riot_id, tag_line, region, summoner_level, profile_icon_id,
		          last_synced_at, created_at, xmax = 0`

	var created bool

	stored, err := scanSummoner(r.pool.QueryRow(ctx, query,
		summoner.PUUID, summoner.RiotID, summoner.TagLine, summoner.Region,
		summoner.SummonerLevel, summoner.ProfileIconID), &created)
	if err != nil {
		return domain.Summoner{}, false, fmt.Errorf("storage: сохранение саммонера: %w", err)
	}

	return stored, created, nil
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

// ByRiotID ищет саммонера по тому же, что принимает POST /summoners, — без puuid.
//
// Нужен ровно для одного случая: Riot недоступен, и резолвить Riot ID в puuid нечем,
// а отдать последний снапшот всё-таки надо (SPEC.md 3.4). Сравнение регистронезависимое:
// пользователь набирает ник руками, а в базе лежит написание, которое вернул Riot.
//
// Строк может оказаться две: после переименования старая запись остаётся со своим
// puuid и старым riot_id, а новая приходит с тем же именем, что набрали. Берём
// синхронизированную позже — она ближе к тому, что человек ожидает увидеть.
func (r *Summoners) ByRiotID(ctx context.Context, region, gameName, tagLine string) (domain.Summoner, error) {
	const query = `
		SELECT puuid, riot_id, tag_line, region, summoner_level, profile_icon_id,
		       last_synced_at, created_at
		FROM summoners
		WHERE lower(region) = lower($1)
		  AND lower(riot_id) = lower($2)
		  AND lower(tag_line) = lower($3)
		ORDER BY last_synced_at DESC NULLS LAST
		LIMIT 1`

	summoner, err := scanSummoner(r.pool.QueryRow(ctx, query, region, gameName, tagLine))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Summoner{}, fmt.Errorf("%w: саммонер %s#%s (%s)", ErrNotFound, gameName, tagLine, region)
		}

		return domain.Summoner{}, fmt.Errorf("storage: чтение саммонера по Riot ID: %w", err)
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

// scanSummoner читает саммонера из строки. Цели из tail читаются после колонок
// саммонера — так Upsert добирает свой признак создания, не дублируя список полей.
func scanSummoner(row scanner, tail ...any) (domain.Summoner, error) {
	var (
		summoner      domain.Summoner
		summonerLevel *int
		profileIconID *int
	)

	targets := []any{
		&summoner.PUUID,
		&summoner.RiotID,
		&summoner.TagLine,
		&summoner.Region,
		&summonerLevel,
		&profileIconID,
		&summoner.LastSyncedAt,
		&summoner.CreatedAt,
	}

	if err := row.Scan(append(targets, tail...)...); err != nil {
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
