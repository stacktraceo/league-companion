package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getMatch отдаёт полные детали матча — обе команды и всех десятерых участников.
//
// Возвращается исходный ответ Match-V5 как есть, из matches.raw_data
// (CLAUDE.md, отклонение 1). Пересобирать его в свои DTO значило бы заново решать,
// какие поля важны, — а колонка заводилась ровно для того, чтобы этот выбор
// не был окончательным: предметы, руны и спеллы уже лежат в ней.
func getMatch(logger *slog.Logger, matches MatchStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := chi.URLParam(r, "matchId")

		raw, err := matches.RawByID(r.Context(), matchID)
		if err != nil {
			respondStorageError(w, r, logger, err,
				"match_not_found", "матч не найден — он появится после синхронизации саммонера")

			return
		}

		respondRawJSON(w, r, logger, http.StatusOK, raw)
	}
}
