package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protectedRoutes — все маршруты под /api/v1: middleware обязан закрывать группу
// целиком, а не только тот эндпоинт, на котором её проверили руками.
func protectedRoutes() []struct {
	method string
	path   string
} {
	return []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/summoners"},
		{http.MethodGet, "/api/v1/summoners/" + testPUUID},
		{http.MethodGet, "/api/v1/summoners/" + testPUUID + "/matches"},
		{http.MethodGet, "/api/v1/matches/EUW1_1"},
	}
}

func TestAPIKeyRejectsRequestWithoutHeader(t *testing.T) {
	router := newTestRouter(t, testDeps())

	for _, route := range protectedRoutes() {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, nil))

			require.Equal(t, http.StatusUnauthorized, rec.Code)

			var body ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "unauthorized", body.Error)
		})
	}
}

func TestAPIKeyRejectsWrongKey(t *testing.T) {
	router := newTestRouter(t, testDeps())

	// Префикс правильного ключа — самый интересный случай: именно на нём обычное
	// сравнение по байтам выдало бы длину совпадения через время ответа.
	for _, key := range []string{"", "другой-секрет", testAPIKey[:len(testAPIKey)-2], testAPIKey + "x"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/summoners/"+testPUUID, nil)
		request.Header.Set(apiKeyHeader, key)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, request)

		assert.Equal(t, http.StatusUnauthorized, rec.Code, "ключ %q не должен подходить", key)
	}
}

func TestAPIKeyAcceptsValidKey(t *testing.T) {
	router := newTestRouter(t, testDeps())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/summoners/"+testPUUID, nil)
	request.Header.Set(apiKeyHeader, testAPIKey)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Health-check обязан работать без секрета: его дёргают docker-compose и мониторинг
// (CLAUDE.md, отклонение 3).
func TestHealthzDoesNotRequireAPIKey(t *testing.T) {
	router := newTestRouter(t, testDeps())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Неавторизованный запрос не должен доходить до хендлера — иначе через POST можно
// было бы бесплатно жечь лимит Riot без ключа.
func TestAPIKeyBlocksHandlerExecution(t *testing.T) {
	deps := testDeps()
	profiles := deps.Profiles.(*fakeProfiles)

	body := bytes.NewBufferString(`{"riotId":"Test","tagLine":"EUW","region":"euw1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/summoners", body)
	request.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	newTestRouter(t, deps).ServeHTTP(rec, request)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Zero(t, profiles.calls, "хендлер не должен был выполниться")
}

// Ключ не должен утечь ни в тело ответа, ни в заголовки.
func TestAPIKeyIsNotLeakedInResponse(t *testing.T) {
	router := newTestRouter(t, testDeps())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/summoners/"+testPUUID, nil)
	request.Header.Set(apiKeyHeader, "почти-верный-ключ")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)

	assert.NotContains(t, rec.Body.String(), testAPIKey)
	assert.NotContains(t, rec.Body.String(), "почти-верный-ключ")
}
