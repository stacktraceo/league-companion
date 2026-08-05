package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const healthCheckTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Logger *slog.Logger
	DB     Pinger

	// ClientAPIKey - shared secret для заголовка X-API-Key (DECISIONS.md, отклонение 3).
	ClientAPIKey string

	Profiles  ProfileSyncer
	Queue     SyncQueue
	Summoners SummonerStore
	Ranked    RankedStore
	Matches   MatchStore

	// Now подменяется в тестах: от настенных часов зависят и граница периода
	// в /stats, и cooldown принудительной синхронизации. Пустое значение - time.Now.
	Now func() time.Time
}

func NewRouter(deps Deps) *chi.Mux {
	now := deps.Now
	if now == nil {
		now = time.Now
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(RequestLogger(deps.Logger))
	router.Use(Recoverer(deps.Logger))

	router.Get("/healthz", healthHandler(deps.Logger, deps.DB))

	router.Route("/api/v1", func(api chi.Router) {
		api.Use(APIKeyAuth(deps.Logger, deps.ClientAPIKey))

		api.Post("/summoners", createSummoner(deps.Logger, deps.Profiles, deps.Queue, deps.Summoners, deps.Ranked))
		api.Get("/summoners/{puuid}", getSummoner(deps.Logger, deps.Summoners, deps.Ranked))
		api.Get("/summoners/{puuid}/matches", listMatches(deps.Logger, deps.Summoners, deps.Matches))
		api.Get("/summoners/{puuid}/stats", getStats(deps.Logger, deps.Summoners, deps.Matches, now))
		api.Post("/summoners/{puuid}/sync", forceSync(deps.Logger, deps.Summoners, deps.Queue, now))
		api.Get("/matches/{matchId}", getMatch(deps.Logger, deps.Matches))
	})

	return router
}

func healthHandler(logger *slog.Logger, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			logger.WarnContext(ctx, "health-check: база недоступна", "error", err)
			respondError(w, r, logger, http.StatusServiceUnavailable,
				"database_unavailable", "база данных недоступна")

			return
		}

		respondJSON(w, r, logger, http.StatusOK, map[string]string{"status": "ok"})
	}
}
