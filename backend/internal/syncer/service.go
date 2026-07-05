// Package syncer синхронизирует данные саммонера из Riot API в PostgreSQL.
//
// Сервис ничего не знает про HTTP: его дёргают и хендлер добавления саммонера,
// и фоновый воркер (SPEC.md 3.5).
package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

// DefaultMatchCount — сколько последних матчей тянуть за один прогон.
const DefaultMatchCount = riot.DefaultMatchIDCount

// RiotClient — то, что синхронизации нужно от клиента Riot.
type RiotClient interface {
	GetAccountByRiotID(ctx context.Context, region, gameName, tagLine string) (*riot.AccountDTO, error)
	GetSummonerByPUUID(ctx context.Context, region, puuid string) (*riot.SummonerDTO, error)
	GetLeagueEntriesByPUUID(ctx context.Context, region, puuid string) ([]riot.LeagueEntryDTO, error)
	GetMatchIDsByPUUID(ctx context.Context, region, puuid string, start, count int) ([]string, error)
	GetMatch(ctx context.Context, region, matchID string) (*riot.MatchDetail, error)
}

// SummonerRepo — хранилище саммонеров.
type SummonerRepo interface {
	// Upsert возвращает true, если саммонер добавлен впервые.
	Upsert(ctx context.Context, summoner domain.Summoner) (bool, error)
	ByPUUID(ctx context.Context, puuid string) (domain.Summoner, error)
	TrackedPUUIDs(ctx context.Context) (map[string]struct{}, error)
	MarkSynced(ctx context.Context, puuid string, at time.Time) error
}

// RankedRepo — хранилище ранговых снапшотов.
type RankedRepo interface {
	Replace(ctx context.Context, puuid string, stats []domain.RankedStat) error
}

// MatchRepo — хранилище матчей.
type MatchRepo interface {
	KnownIDs(ctx context.Context, ids []string) (map[string]struct{}, error)
	Insert(ctx context.Context, match domain.Match, participants []domain.MatchParticipant) error
}

// Service синхронизирует одного саммонера.
type Service struct {
	riot      RiotClient
	summoners SummonerRepo
	ranked    RankedRepo
	matches   MatchRepo
	logger    *slog.Logger

	// now вынесен в поле, чтобы тесты не зависели от настенных часов.
	now func() time.Time
}

func NewService(
	client RiotClient,
	summoners SummonerRepo,
	ranked RankedRepo,
	matches MatchRepo,
	logger *slog.Logger,
) *Service {
	return &Service{
		riot:      client,
		summoners: summoners,
		ranked:    ranked,
		matches:   matches,
		logger:    logger,
		now:       time.Now,
	}
}

// SyncProfile резолвит Riot ID в PUUID и сохраняет профиль с рангами.
// Второе значение — true, если саммонер добавлен впервые.
//
// Три запроса к Riot: Account-V1 (regional), Summoner-V4 и League-V4 (platform).
func (s *Service) SyncProfile(
	ctx context.Context,
	region, gameName, tagLine string,
) (domain.Summoner, bool, error) {
	account, err := s.riot.GetAccountByRiotID(ctx, region, gameName, tagLine)
	if err != nil {
		return domain.Summoner{}, false, fmt.Errorf("резолв Riot ID %s#%s: %w", gameName, tagLine, err)
	}

	profile, err := s.riot.GetSummonerByPUUID(ctx, region, account.PUUID)
	if err != nil {
		return domain.Summoner{}, false, fmt.Errorf("профиль саммонера: %w", err)
	}

	summoner := domain.SummonerFromRiot(*account, *profile, region)

	created, err := s.summoners.Upsert(ctx, summoner)
	if err != nil {
		return domain.Summoner{}, false, err
	}

	s.syncRanks(ctx, summoner)

	return summoner, created, nil
}

// syncRanks обновляет ранговый снапшот.
//
// Ошибка League-V4 намеренно не роняет добавление саммонера: профиль — то, ради
// чего пользователь пришёл, а ранг подтянется следующей синхронизацией. Пустой
// список рангов — валидный ответ для безрангового игрока, а не сбой.
func (s *Service) syncRanks(ctx context.Context, summoner domain.Summoner) {
	entries, err := s.riot.GetLeagueEntriesByPUUID(ctx, summoner.Region, summoner.PUUID)
	if err != nil {
		s.logger.WarnContext(ctx, "не удалось получить ранги, профиль сохранён без них",
			"puuid", summoner.PUUID, "error", err)

		return
	}

	stats := domain.RankedStatsFromRiot(summoner.PUUID, entries, s.now())

	if err := s.ranked.Replace(ctx, summoner.PUUID, stats); err != nil {
		s.logger.WarnContext(ctx, "не удалось сохранить ранги", "puuid", summoner.PUUID, "error", err)
	}
}

// SyncSummoner подтягивает новые матчи саммонера по его puuid.
func (s *Service) SyncSummoner(ctx context.Context, puuid string, count int) (int, error) {
	summoner, err := s.summoners.ByPUUID(ctx, puuid)
	if err != nil {
		return 0, err
	}

	return s.SyncMatches(ctx, summoner, count)
}

// SyncMatches сохраняет матчи саммонера, которых ещё нет в БД, и возвращает их число.
func (s *Service) SyncMatches(ctx context.Context, summoner domain.Summoner, count int) (int, error) {
	if count <= 0 {
		count = DefaultMatchCount
	}

	ids, err := s.riot.GetMatchIDsByPUUID(ctx, summoner.Region, summoner.PUUID, 0, count)
	if err != nil {
		return 0, fmt.Errorf("список матчей: %w", err)
	}

	// Детали матча весят ~140 КБ; перекачивать уже сохранённые — впустую жечь
	// лимит ключа.
	known, err := s.matches.KnownIDs(ctx, ids)
	if err != nil {
		return 0, err
	}

	// На match_participants стоит FK на summoners, поэтому чужие участники
	// в таблицу всё равно не лягут (CLAUDE.md, отклонение 1).
	tracked, err := s.summoners.TrackedPUUIDs(ctx)
	if err != nil {
		return 0, err
	}

	synced := 0

	for _, id := range ids {
		if _, ok := known[id]; ok {
			continue
		}

		if err := s.syncMatch(ctx, summoner.Region, id, tracked); err != nil {
			// Протухший ключ и отменённый контекст не лечатся следующим матчем.
			if errors.Is(err, riot.ErrUnauthorized) || ctx.Err() != nil {
				return synced, err
			}

			s.logger.WarnContext(ctx, "матч не синхронизирован, продолжаю",
				"match_id", id, "puuid", summoner.PUUID, "error", err)

			continue
		}

		synced++
	}

	if err := s.summoners.MarkSynced(ctx, summoner.PUUID, s.now()); err != nil {
		return synced, err
	}

	s.logger.InfoContext(ctx, "синхронизация завершена",
		"puuid", summoner.PUUID, "новых матчей", synced, "проверено", len(ids))

	return synced, nil
}

func (s *Service) syncMatch(ctx context.Context, region, matchID string, tracked map[string]struct{}) error {
	detail, err := s.riot.GetMatch(ctx, region, matchID)
	if err != nil {
		return err
	}

	match, err := domain.MatchFromRiot(*detail)
	if err != nil {
		return err
	}

	participants, err := domain.MatchParticipantsFromRiot(*detail)
	if err != nil {
		return err
	}

	return s.matches.Insert(ctx, match, trackedOnly(participants, tracked))
}

// trackedOnly оставляет участие только тех, кого мы отслеживаем. Полный состав
// обеих команд остаётся доступен из matches.raw_data.
func trackedOnly(participants []domain.MatchParticipant, tracked map[string]struct{}) []domain.MatchParticipant {
	filtered := make([]domain.MatchParticipant, 0, len(tracked))

	for _, participant := range participants {
		if _, ok := tracked[participant.PUUID]; ok {
			filtered = append(filtered, participant)
		}
	}

	return filtered
}
