package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

const apiKeyHeader = "X-API-Key"

func APIKeyAuth(logger *slog.Logger, expected string) func(http.Handler) http.Handler {
	want := []byte(expected)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get(apiKeyHeader))

			if subtle.ConstantTimeCompare(got, want) != 1 {
				logger.WarnContext(r.Context(), "запрос без валидного X-API-Key",
					"request_id", middleware.GetReqID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"ключ_передан", len(got) > 0,
				)

				respondError(w, r, logger, http.StatusUnauthorized,
					"unauthorized", "требуется валидный заголовок "+apiKeyHeader)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
