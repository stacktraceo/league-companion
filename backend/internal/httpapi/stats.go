package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Параметры агрегации статистики (SPEC.md 3.4).
const (
	// defaultPeriodDays — период по умолчанию, тот же, что в примере SPEC.md 3.4.
	defaultPeriodDays = 30

	// maxPeriodDays — год. Дальше смысла нет: сезоны короче, а матчи копятся
	// только с момента добавления саммонера.
	maxPeriodDays = 365

	// topChampionCount — «топ-5 чемпионов по количеству игр» из SPEC.md 3.4.
	topChampionCount = 5
)

// getStats отдаёт агрегацию за период: винрейт, KDA и топ-5 чемпионов.
//
// Считает по данным из Postgres и в Riot не ходит: свежесть обеспечивает фоновая
// синхронизация, а укладываться нужно в 200 мс (SPEC.md 3.6).
func getStats(logger *slog.Logger, summoners SummonerStore, matches MatchStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		puuid := chi.URLParam(r, "puuid")

		days, err := parsePeriod(r.URL.Query().Get("period"))
		if err != nil {
			respondError(w, r, logger, http.StatusBadRequest, "invalid_period", err.Error())

			return
		}

		// «Саммонер не отслеживается» и «не играл за период» — разные ответы,
		// поэтому саммонера проверяем отдельно.
		if _, err := summoners.ByPUUID(r.Context(), puuid); err != nil {
			respondStorageError(w, r, logger, err, "summoner_not_found", "саммонер не отслеживается")

			return
		}

		since := now().AddDate(0, 0, -days)

		participations, err := matches.ParticipationsSince(r.Context(), puuid, since)
		if err != nil {
			respondStorageError(w, r, logger, err, "summoner_not_found", "саммонер не найден")

			return
		}

		// Пустой период — это 200 с нулями, а не 404: саммонер существует,
		// играл он или нет — другой вопрос.
		respondJSON(w, r, logger, http.StatusOK, statsResponse(participations, days, since))
	}
}

// parsePeriod разбирает ?period=30d и возвращает число дней.
//
// Пустое значение берёт период по умолчанию, некорректное — отвергается: молча
// подставить свой период значило бы отдать клиенту цифры не за то, что он просил.
func parsePeriod(raw string) (int, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return defaultPeriodDays, nil
	}

	digits, ok := strings.CutSuffix(raw, "d")
	if !ok {
		return 0, errors.New("period задаётся в днях, например 30d")
	}

	days, err := strconv.Atoi(digits)
	if err != nil {
		return 0, errors.New("period: " + strconv.Quote(raw) + " не число дней, например 30d")
	}

	if days < 1 || days > maxPeriodDays {
		return 0, errors.New("period должен быть в диапазоне 1d.." + strconv.Itoa(maxPeriodDays) + "d")
	}

	return days, nil
}
