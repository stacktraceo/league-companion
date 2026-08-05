package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getMatch отдаёт полные детали матча — обе команды и всех десятерых участников.
//
// Возвращается ответ Match-V5 из matches.raw_data без пересборки в свои DTO
// (DECISIONS.md, отклонение 1): иначе пришлось бы заново решать, какие поля важны,
// а колонка заводилась ровно для того, чтобы этот выбор не был окончательным —
// предметы, руны и спеллы уже лежат в ней.
//
// «Без пересборки» — с точностью до нормализации jsonb: Postgres хранит документ
// в разобранном виде и на выходе даёт свой порядок ключей и свои пробелы. Значения
// при этом те же, а побайтовое совпадение с ответом Riot никому не нужно.
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
