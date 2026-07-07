package httpapi

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/stacktraceo/league-companion/backend/internal/syncer"
)

// manualSyncCooldown — минимальный интервал между принудительными синхронизациями
// одного саммонера (SPEC.md 3.4 требует «не чаще раза в N минут»).
//
// Две минуты — это окно второго лимита Riot (100 запросов за 2 минуты, SPEC.md 3.2),
// а один полный прогон стоит около двадцати запросов. Отсчёт идёт от last_synced_at:
// отдельного состояния в памяти не заводим — оно бы не переживало рестарт, а
// last_synced_at и означает ровно «данные настолько свежие».
const manualSyncCooldown = 2 * time.Minute

// forceSync ставит саммонера в очередь на внеплановую синхронизацию.
//
// Отвечает 202, а не 200: прогон — это около двадцати запросов к Riot под лимитом,
// держать ради него соединение незачем. Результат виден в логах и в lastSyncedAt.
func forceSync(logger *slog.Logger, summoners SummonerStore, queue SyncQueue, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		puuid := chi.URLParam(r, "puuid")

		summoner, err := summoners.ByPUUID(r.Context(), puuid)
		if err != nil {
			respondStorageError(w, r, logger, err,
				"summoner_not_found", "саммонер не отслеживается — сначала добавь его через POST /api/v1/summoners")

			return
		}

		if retryAfter, tooSoon := cooldownLeft(summoner.LastSyncedAt, now()); tooSoon {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			respondError(w, r, logger, http.StatusTooManyRequests, "sync_too_soon",
				"синхронизация была меньше "+manualSyncCooldown.String()+" назад, попробуй позже")

			return
		}

		if !queue.Enqueue(puuid, syncer.DefaultMatchCount) {
			// Очередь полна — запрос ничего не сделал, и клиенту надо об этом сказать.
			// Для POST /summoners это было бы не так: там профиль уже сохранён.
			logger.WarnContext(r.Context(), "принудительная синхронизация не поставлена в очередь",
				"puuid", puuid)
			respondError(w, r, logger, http.StatusServiceUnavailable, "sync_queue_full",
				"очередь синхронизации переполнена, попробуй позже")

			return
		}

		respondJSON(w, r, logger, http.StatusAccepted, syncAcceptedResponse(puuid, summoner.LastSyncedAt))
	}
}

// cooldownLeft сообщает, сколько секунд осталось ждать до следующей разрешённой
// синхронизации. Ни разу не синхронизированный саммонер ждать не должен.
func cooldownLeft(lastSyncedAt *time.Time, now time.Time) (int, bool) {
	if lastSyncedAt == nil {
		return 0, false
	}

	left := manualSyncCooldown - now.Sub(*lastSyncedAt)
	if left <= 0 {
		return 0, false
	}

	// Округление вверх: Retry-After в секундах, и просить подождать 0 секунд,
	// когда осталось полсекунды, — врать клиенту.
	return int(math.Ceil(left.Seconds())), true
}
