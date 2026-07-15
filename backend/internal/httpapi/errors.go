package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/stacktraceo/league-companion/backend/internal/riot"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
)

// respondUpstreamError переводит ошибку синхронизации в ответ API (SPEC.md 3.4).
//
// notFoundCode задаёт вызывающий: «не найден» для саммонера и для матча — разные
// ситуации, и клиенту полезно их различать.
func respondUpstreamError(
	w http.ResponseWriter,
	r *http.Request,
	logger *slog.Logger,
	err error,
	notFoundCode, notFoundMessage string,
) {
	var rateLimitErr *riot.RateLimitError

	switch {
	case errors.Is(err, riot.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		respondError(w, r, logger, http.StatusNotFound, notFoundCode, notFoundMessage)

	case errors.Is(err, riot.ErrUnknownRegion):
		respondError(w, r, logger, http.StatusBadRequest, "invalid_region",
			"неизвестный регион, допустимые: "+regionList())

	case errors.As(err, &rateLimitErr):
		// Retry-After наружу отдаём как есть: клиенту нужен тот же срок, что назвал Riot.
		w.Header().Set("Retry-After", strconv.Itoa(int(rateLimitErr.RetryAfter.Seconds())))
		logger.WarnContext(r.Context(), "упёрлись в лимит Riot", "error", err)
		respondError(w, r, logger, http.StatusTooManyRequests, "rate_limited",
			"превышен лимит запросов к Riot API, попробуй позже")

	case errors.Is(err, riot.ErrUnauthorized):
		// Это не про клиента: отклонён наш ключ, скорее всего протухший
		// (dev-ключ живёт 24 часа), поэтому 502, а не 401.
		logger.ErrorContext(r.Context(), "Riot отклонил ключ — проверь RIOT_API_KEY", "error", err)
		respondError(w, r, logger, http.StatusBadGateway, "riot_unauthorized",
			"Riot API отклонил ключ бэкенда")

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		logger.WarnContext(r.Context(), "запрос к Riot не уложился в отведённое время", "error", err)
		respondError(w, r, logger, http.StatusGatewayTimeout, "riot_timeout",
			"Riot API не ответил вовремя")

	default:
		logger.ErrorContext(r.Context(), "ошибка обращения к Riot API", "error", err)
		respondError(w, r, logger, http.StatusBadGateway, "riot_unavailable",
			"Riot API недоступен")
	}
}

// riotUnavailable сообщает, что до Riot не дозвонились, — то есть отдать
// сохранённый снапшот уместно (SPEC.md 3.4).
//
// Это не «любая ошибка»: 404 и неизвестный регион означают, что отдавать нечего по
// существу, а 429 SPEC перечисляет отдельно от клаузы про stale — там надо подождать
// названный Riot срок, а не показывать старые данные. Таймаут формально уходит как
// 504, а не 502, но ситуация та же самая, и держать для неё худшее поведение не за что.
func riotUnavailable(err error) bool {
	var rateLimitErr *riot.RateLimitError

	switch {
	case errors.Is(err, riot.ErrNotFound), errors.Is(err, storage.ErrNotFound),
		errors.Is(err, riot.ErrUnknownRegion),
		errors.As(err, &rateLimitErr):
		return false

	default:
		return true
	}
}

// respondStorageError отвечает на ошибку чтения из БД.
func respondStorageError(
	w http.ResponseWriter,
	r *http.Request,
	logger *slog.Logger,
	err error,
	notFoundCode, notFoundMessage string,
) {
	if errors.Is(err, storage.ErrNotFound) {
		respondError(w, r, logger, http.StatusNotFound, notFoundCode, notFoundMessage)

		return
	}

	logger.ErrorContext(r.Context(), "ошибка работы с базой", "error", err)
	respondError(w, r, logger, http.StatusInternalServerError, "internal_error",
		"внутренняя ошибка сервера")
}
