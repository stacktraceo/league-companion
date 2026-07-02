// Package httpapi собирает HTTP-слой бэкенда: роутер, middleware и хендлеры.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// healthCheckTimeout ограничивает пинг БД, чтобы health-check не висел.
const healthCheckTimeout = 2 * time.Second

// Pinger — то, что умеет проверять доступность (реализуется *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter собирает роутер бэкенда.
//
// /healthz намеренно живёт вне /api/v1: middleware проверки X-API-Key,
// которое появится в вехе 5–6, вешается только на /api/v1/*
// (CLAUDE.md, отклонение 3).
func NewRouter(logger *slog.Logger, db Pinger) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(RequestLogger(logger))
	router.Use(Recoverer(logger))

	router.Get("/healthz", healthHandler(logger, db))

	return router
}

// healthHandler отвечает 200, если бэкенд жив и база отвечает, иначе 503.
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
