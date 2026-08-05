package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthzOK(t *testing.T) {
	router := newHealthRouter(t, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealthzReportsDatabaseFailure(t *testing.T) {
	router := newHealthRouter(t, stubPinger{err: errors.New("connection refused")})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "database_unavailable", body.Error)
	assert.NotEmpty(t, body.Message)

	// Детали внутренней ошибки наружу не отдаём.
	assert.NotContains(t, rec.Body.String(), "connection refused")
}

func TestRequestIDHeaderIsSet(t *testing.T) {
	router := newHealthRouter(t, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}

func TestUnknownRouteReturns404(t *testing.T) {
	router := newHealthRouter(t, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/такого-маршрута-нет", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRecovererCatchesPanic(t *testing.T) {
	router := newHealthRouter(t, stubPinger{})
	router.Get("/boom", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Error)

	// Ни текст паники, ни стек наружу не уходят - только в лог.
	assert.NotContains(t, rec.Body.String(), "boom")
}

func TestRecovererRepanicsOnAbortHandler(t *testing.T) {
	router := newHealthRouter(t, stubPinger{})
	router.Get("/abort", func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})

	rec := httptest.NewRecorder()
	assert.PanicsWithError(t, http.ErrAbortHandler.Error(), func() {
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/abort", nil))
	})
}
