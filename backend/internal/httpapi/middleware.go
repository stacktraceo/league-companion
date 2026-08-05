package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const requestIDHeader = "X-Request-Id"

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := middleware.GetReqID(r.Context())
			if requestID != "" {
				w.Header().Set(requestIDHeader, requestID)
			}

			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()

			//nolint:contextcheck // ложное срабатывание: r.Context() передаётся в LogAttrs, но contextcheck не видит его сквозь отложенное замыкание
			defer func() {
				logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
					slog.String("request_id", requestID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", wrapped.Status()),
					slog.Int("bytes", wrapped.BytesWritten()),
					slog.Duration("duration", time.Since(started)),
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//nolint:contextcheck // ложное срабатывание: r.Context() передаётся в LogAttrs ниже
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				// http.ErrAbortHandler - штатный способ оборвать ответ,
				// его пробрасываем дальше по конвенции net/http.
				if recovered == http.ErrAbortHandler { //nolint:errorlint // сравнение с sentinel-значением паники
					panic(recovered)
				}

				logger.LogAttrs(r.Context(), slog.LevelError, "паника в хендлере",
					slog.String("request_id", middleware.GetReqID(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("panic", recovered),
					slog.String("stack", string(debug.Stack())),
				)

				respondError(w, r, logger, http.StatusInternalServerError,
					"internal_error", "внутренняя ошибка сервера")
			}()

			next.ServeHTTP(w, r)
		})
	}
}
