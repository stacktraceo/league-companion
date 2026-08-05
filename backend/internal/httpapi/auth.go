package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// apiKeyHeader — заголовок shared secret между Android-клиентом и бэкендом
// (DECISIONS.md, отклонение 3).
const apiKeyHeader = "X-API-Key"

// APIKeyAuth проверяет X-API-Key на всех запросах, которые через него проходят.
//
// Вешается только на /api/v1/*: health-check должен оставаться доступным
// мониторингу и docker-compose без секрета.
//
// Сравнение — constant-time: обычное == выходит на первом различающемся байте,
// и по времени ответа ключ подбирается посимвольно. Сам ключ никуда не логируется.
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
