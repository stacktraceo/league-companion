package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse — единый формат ошибок API (SPEC.md 3.4).
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	// Stale выставляется, когда Riot недоступен, но есть закэшированные данные.
	Stale bool `json:"stale,omitempty"`
}

// respondJSON сериализует payload и отдаёт его со статусом status.
func respondJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Статус уже ушёл в сеть — остаётся только записать это в лог.
		logger.ErrorContext(r.Context(), "не удалось сериализовать ответ", "error", err)
	}
}

// respondRawJSON отдаёт готовый JSON без пересериализации — так сырой ответ
// Match-V5 доходит до клиента ровно в том виде, в каком его прислал Riot.
func respondRawJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write(payload); err != nil {
		logger.ErrorContext(r.Context(), "не удалось записать ответ", "error", err)
	}
}

// respondError отдаёт ошибку в едином формате.
func respondError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, code, message string) {
	respondJSON(w, r, logger, status, ErrorResponse{Error: code, Message: message})
}
