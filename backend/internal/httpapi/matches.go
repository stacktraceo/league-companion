package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func getMatch(logger *slog.Logger, matches MatchStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := chi.URLParam(r, "matchId")

		raw, err := matches.RawByID(r.Context(), matchID)
		if err != nil {
			respondStorageError(w, r, logger, err,
				"match_not_found", "матч не найден - он появится после синхронизации саммонера")

			return
		}

		respondRawJSON(w, r, logger, http.StatusOK, raw)
	}
}
