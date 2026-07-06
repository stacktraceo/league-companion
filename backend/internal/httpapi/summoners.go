package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
	"github.com/stacktraceo/league-companion/backend/internal/syncer"
)

// Пагинация ленты матчей (SPEC.md 3.4).
const (
	defaultLimit = 20
	maxLimit     = 100

	// maxRequestBody — тело POST /summoners крошечное; читать больше незачем.
	maxRequestBody = 4 << 10
)

// ProfileSyncer резолвит Riot ID и сохраняет профиль.
type ProfileSyncer interface {
	SyncProfile(ctx context.Context, region, gameName, tagLine string) (domain.Summoner, bool, error)
}

// SyncQueue ставит саммонера в очередь на загрузку матчей.
type SyncQueue interface {
	Enqueue(puuid string, count int) bool
}

// SummonerStore — чтение саммонеров.
type SummonerStore interface {
	ByPUUID(ctx context.Context, puuid string) (domain.Summoner, error)
}

// RankedStore — чтение ранговых снапшотов.
type RankedStore interface {
	ByPUUID(ctx context.Context, puuid string) ([]domain.RankedStat, error)
}

// MatchStore — чтение матчей.
type MatchStore interface {
	ListByPUUID(ctx context.Context, puuid string, limit, offset int) ([]storage.MatchListItem, error)
	CountByPUUID(ctx context.Context, puuid string) (int, error)
	RawByID(ctx context.Context, matchID string) (json.RawMessage, error)
}

// createSummoner добавляет саммонера по Riot ID.
//
// Профиль и ранг тянутся синхронно — три запроса к Riot, около полутора секунд.
// Матчи (два десятка запросов) уходят в фон: держать ради них HTTP-соединение
// на полминуты незачем, а саммонер уже сохранён и доступен (SPEC.md 3.4).
func createSummoner(logger *slog.Logger, service ProfileSyncer, queue SyncQueue, ranked RankedStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request createSummonerRequest

		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&request); err != nil {
			respondError(w, r, logger, http.StatusBadRequest, "invalid_body",
				"не удалось разобрать тело запроса: ожидается {riotId, tagLine, region}")

			return
		}

		gameName := strings.TrimSpace(request.RiotID)
		tagLine := strings.TrimSpace(strings.TrimPrefix(request.TagLine, "#"))
		region := strings.ToLower(strings.TrimSpace(request.Region))

		if message, ok := validateCreateRequest(gameName, tagLine, region); !ok {
			respondError(w, r, logger, http.StatusBadRequest, "invalid_request", message)

			return
		}

		summoner, created, err := service.SyncProfile(r.Context(), region, gameName, tagLine)
		if err != nil {
			respondUpstreamError(w, r, logger, err,
				"summoner_not_found", "саммонер с таким Riot ID не найден")

			return
		}

		if !queue.Enqueue(summoner.PUUID, syncer.DefaultMatchCount) {
			// Не ошибка: профиль сохранён, матчи подтянет следующий прогон.
			logger.WarnContext(r.Context(), "матчи не поставлены в очередь", "puuid", summoner.PUUID)
		}

		stats, err := ranked.ByPUUID(r.Context(), summoner.PUUID)
		if err != nil {
			logger.WarnContext(r.Context(), "не удалось прочитать ранги после сохранения",
				"puuid", summoner.PUUID, "error", err)
		}

		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}

		respondJSON(w, r, logger, status, summonerResponse(summoner, stats, false))
	}
}

func validateCreateRequest(gameName, tagLine, region string) (string, bool) {
	switch {
	case gameName == "":
		return "riotId обязателен — это игровое имя без тега", false

	case strings.Contains(gameName, "#"):
		return "riotId — только игровое имя; тег передаётся отдельно в tagLine", false

	case tagLine == "":
		return "tagLine обязателен — это часть Riot ID после «#»", false

	case region == "":
		return "region обязателен, допустимые: " + regionList(), false
	}

	// Проверяем регион до похода в Riot: platform-регион легко перепутать
	// с regional-маршрутом вроде «europe» (риск из SPEC.md 7).
	if _, err := riot.PlatformHost(region); err != nil {
		return "неизвестный регион " + strconv.Quote(region) + ", допустимые: " + regionList(), false
	}

	return "", true
}

func regionList() string {
	return strings.Join(riot.SupportedRegions(), ", ")
}

// getSummoner отдаёт профиль с рангами из БД.
//
// В Riot не ходим вовсе: ответ по закэшированным данным обязан укладываться
// в 200 мс (SPEC.md 3.6), а свежесть обеспечивает фоновая синхронизация.
func getSummoner(logger *slog.Logger, summoners SummonerStore, ranked RankedStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		puuid := chi.URLParam(r, "puuid")

		summoner, err := summoners.ByPUUID(r.Context(), puuid)
		if err != nil {
			respondStorageError(w, r, logger, err,
				"summoner_not_found", "саммонер не отслеживается — сначала добавь его через POST /api/v1/summoners")

			return
		}

		stats, err := ranked.ByPUUID(r.Context(), puuid)
		if err != nil {
			respondStorageError(w, r, logger, err, "summoner_not_found", "саммонер не найден")

			return
		}

		respondJSON(w, r, logger, http.StatusOK, summonerResponse(summoner, stats, false))
	}
}

// listMatches отдаёт страницу ленты матчей саммонера.
func listMatches(logger *slog.Logger, summoners SummonerStore, matches MatchStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		puuid := chi.URLParam(r, "puuid")

		limit, offset, err := parsePagination(r)
		if err != nil {
			respondError(w, r, logger, http.StatusBadRequest, "invalid_pagination", err.Error())

			return
		}

		// Отличаем «саммонер не отслеживается» от «матчей пока нет»: для клиента
		// это разные ситуации.
		if _, err := summoners.ByPUUID(r.Context(), puuid); err != nil {
			respondStorageError(w, r, logger, err,
				"summoner_not_found", "саммонер не отслеживается")

			return
		}

		items, err := matches.ListByPUUID(r.Context(), puuid, limit, offset)
		if err != nil {
			respondStorageError(w, r, logger, err, "summoner_not_found", "саммонер не найден")

			return
		}

		total, err := matches.CountByPUUID(r.Context(), puuid)
		if err != nil {
			respondStorageError(w, r, logger, err, "summoner_not_found", "саммонер не найден")

			return
		}

		respondJSON(w, r, logger, http.StatusOK, matchListResponse(items, limit, offset, total))
	}
}

// parsePagination разбирает limit и offset. Отсутствующие параметры берут значения
// по умолчанию, некорректные — отвергаются: молча подставлять свои числа хуже,
// чем сказать клиенту, что он ошибся.
func parsePagination(r *http.Request) (limit, offset int, err error) {
	query := r.URL.Query()

	limit = defaultLimit
	if raw := query.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, errors.New("limit должен быть числом")
		}

		if limit < 1 || limit > maxLimit {
			return 0, 0, errors.New("limit должен быть в диапазоне 1.." + strconv.Itoa(maxLimit))
		}
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, errors.New("offset должен быть числом")
		}

		if offset < 0 {
			return 0, 0, errors.New("offset не может быть отрицательным")
		}
	}

	return limit, offset, nil
}
