package riot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAPIKey = "RGAPI-not-a-real-key"
	testPUUID  = "test-puuid-0000000000000000000000000000000000000000000000000000000000000000000"
)

func fixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)

	return string(data)
}

// newTestClient поднимает httptest-сервер с заданным обработчиком и возвращает
// клиент, направленный на него.
//
// Повторы по умолчанию выключены: тесты ниже проверяют разбор одного ответа, а
// с ретраями они ждали бы настоящий backoff. Сами повторы проверяются в retry_test.go.
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	options := append([]Option{WithBaseURL(server.URL), WithRetry(1, 0)}, opts...)

	return New(testAPIKey, options...)
}

// jsonHandler отдаёт готовое тело и записывает пришедший запрос.
func jsonHandler(t *testing.T, body string, captured **http.Request) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			clone := r.Clone(context.Background())
			*captured = clone
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

func TestClientSendsAPIKeyHeader(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, jsonHandler(t, fixture(t, "account.json"), &got))

	_, err := client.GetAccountByRiotID(context.Background(), "euw1", "Test Summoner", "EUW")
	require.NoError(t, err)

	require.NotNil(t, got)
	assert.Equal(t, testAPIKey, got.Header.Get(apiKeyHeader))
	assert.Equal(t, "application/json", got.Header.Get("Accept"))

	// Ключ не должен утекать в query-параметры — только заголовок.
	assert.NotContains(t, got.URL.RawQuery, testAPIKey)
}

func TestGetAccountByRiotID(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, jsonHandler(t, fixture(t, "account.json"), &got))

	account, err := client.GetAccountByRiotID(context.Background(), "euw1", "Test Summoner", "EUW")
	require.NoError(t, err)

	assert.Equal(t, testPUUID, account.PUUID)
	assert.Equal(t, "Test Summoner", account.GameName)
	assert.Equal(t, "EUW", account.TagLine)

	// Пробел в игровом имени должен быть экранирован, а не разорвать путь.
	require.NotNil(t, got)
	assert.Equal(t, "/riot/account/v1/accounts/by-riot-id/Test%20Summoner/EUW", got.URL.EscapedPath())
	assert.Equal(t, "/riot/account/v1/accounts/by-riot-id/Test Summoner/EUW", got.URL.Path)
}

func TestGetAccountByRiotIDValidation(t *testing.T) {
	client := New(testAPIKey, WithBaseURL("http://127.0.0.1:1"))

	_, err := client.GetAccountByRiotID(context.Background(), "euw1", "", "EUW")
	assert.ErrorIs(t, err, ErrEmptyRiotID)

	_, err = client.GetAccountByRiotID(context.Background(), "euw1", "Name", "")
	assert.ErrorIs(t, err, ErrEmptyRiotID)
}

func TestGetSummonerByPUUID(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, jsonHandler(t, fixture(t, "summoner.json"), &got))

	summoner, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)
	require.NoError(t, err)

	assert.Equal(t, testPUUID, summoner.PUUID)
	assert.Equal(t, 5678, summoner.ProfileIconID)
	assert.Equal(t, int64(412), summoner.SummonerLevel)

	require.NotNil(t, got)
	assert.Equal(t, "/lol/summoner/v4/summoners/by-puuid/"+testPUUID, got.URL.Path)
}

func TestGetLeagueEntriesByPUUID(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, jsonHandler(t, fixture(t, "league_entries.json"), &got))

	entries, err := client.GetLeagueEntriesByPUUID(context.Background(), "ru", testPUUID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "RANKED_SOLO_5x5", entries[0].QueueType)
	assert.Equal(t, "GOLD", entries[0].Tier)
	assert.Equal(t, "II", entries[0].Rank)
	assert.Equal(t, 47, entries[0].LeaguePoints)
	assert.Equal(t, 63, entries[0].Wins)
	assert.Equal(t, 58, entries[0].Losses)
	assert.Equal(t, "RANKED_FLEX_SR", entries[1].QueueType)

	require.NotNil(t, got)
	assert.Equal(t, "/lol/league/v4/entries/by-puuid/"+testPUUID, got.URL.Path)
}

// У безранговых саммонеров League-V4 отдаёт пустой массив — это валидный ответ.
func TestGetLeagueEntriesUnranked(t *testing.T) {
	client := newTestClient(t, jsonHandler(t, "[]", nil))

	entries, err := client.GetLeagueEntriesByPUUID(context.Background(), "ru", testPUUID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGetMatchIDsByPUUID(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, jsonHandler(t, fixture(t, "match_ids.json"), &got))

	ids, err := client.GetMatchIDsByPUUID(context.Background(), "euw1", testPUUID, 20, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"EUW1_7000000001", "EUW1_7000000002", "EUW1_7000000003"}, ids)

	require.NotNil(t, got)
	assert.Equal(t, "/lol/match/v5/matches/by-puuid/"+testPUUID+"/ids", got.URL.Path)
	assert.Equal(t, "20", got.URL.Query().Get("start"))
	assert.Equal(t, "10", got.URL.Query().Get("count"))
}

func TestGetMatchIDsValidation(t *testing.T) {
	// Порт 1 гарантирует отказ соединения, если валидация вдруг пропустит запрос.
	client := New(testAPIKey, WithBaseURL("http://127.0.0.1:1"))
	ctx := context.Background()

	_, err := client.GetMatchIDsByPUUID(ctx, "euw1", "", 0, 20)
	assert.ErrorIs(t, err, ErrEmptyPUUID)

	_, err = client.GetMatchIDsByPUUID(ctx, "euw1", testPUUID, -1, 20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start")

	for _, count := range []int{0, -5, MaxMatchIDCount + 1} {
		_, err = client.GetMatchIDsByPUUID(ctx, "euw1", testPUUID, 0, count)
		require.Error(t, err, "count=%d должен отклоняться", count)
		assert.Contains(t, err.Error(), "count")
	}
}

func TestGetMatch(t *testing.T) {
	raw := fixture(t, "match.json")

	var got *http.Request
	client := newTestClient(t, jsonHandler(t, raw, &got))

	detail, err := client.GetMatch(context.Background(), "euw1", "EUW1_7000000001")
	require.NoError(t, err)

	assert.Equal(t, "EUW1_7000000001", detail.Match.Metadata.MatchID)
	assert.Equal(t, int64(1700000000000), detail.Match.Info.GameCreation)
	assert.Equal(t, int64(1834), detail.Match.Info.GameDuration)
	assert.Equal(t, "14.1.556.1234", detail.Match.Info.GameVersion)
	assert.Equal(t, 420, detail.Match.Info.QueueID)
	require.Len(t, detail.Match.Info.Participants, 2)

	first := detail.Match.Info.Participants[0]
	assert.Equal(t, "Ahri", first.ChampionName)
	assert.Equal(t, 11, first.Kills)
	assert.Equal(t, 3, first.Deaths)
	assert.Equal(t, 9, first.Assists)
	assert.True(t, first.Win)
	assert.Equal(t, 14320, first.GoldEarned)
	assert.Equal(t, 201, first.CS(), "CS = миньоны + лесные монстры")

	require.NotNil(t, got)
	assert.Equal(t, "/lol/match/v5/matches/EUW1_7000000001", got.URL.Path)
}

// Сырой JSON обязан сохраняться байт-в-байт: из него наполняется matches.raw_data,
// и в нём должны остаться поля, которых нет в DTO (CLAUDE.md, отклонение 1).
func TestGetMatchPreservesRawJSON(t *testing.T) {
	raw := fixture(t, "match.json")
	client := newTestClient(t, jsonHandler(t, raw, nil))

	detail, err := client.GetMatch(context.Background(), "euw1", "EUW1_7000000001")
	require.NoError(t, err)

	assert.JSONEq(t, raw, string(detail.Raw))
	assert.Equal(t, raw, string(detail.Raw), "тело должно сохраняться без пересериализации")

	// Поля, которые DTO не разбирает, но которые пригодятся при расширении схемы.
	assert.Contains(t, string(detail.Raw), "perks")
	assert.Contains(t, string(detail.Raw), "item0")
	assert.Contains(t, string(detail.Raw), "teams")

	assert.True(t, json.Valid(detail.Raw))
}

func TestGetMatchValidation(t *testing.T) {
	client := New(testAPIKey, WithBaseURL("http://127.0.0.1:1"))

	_, err := client.GetMatch(context.Background(), "euw1", "")
	assert.ErrorIs(t, err, ErrEmptyMatchID)
}

func TestNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"status":{"message":"Data not found","status_code":404}}`)
	})

	_, err := client.GetAccountByRiotID(context.Background(), "euw1", "Nobody", "XXX")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.False(t, IsRetryable(err), "404 повторять бессмысленно")
}

// 401/403 на dev-ключе почти всегда означает, что ключ протух (SPEC.md 3.2).
func TestUnauthorized(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			})

			_, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)
			assert.ErrorIs(t, err, ErrUnauthorized)
			assert.False(t, IsRetryable(err), "протухший ключ ретраями не чинится")
		})
	}
}

func TestRateLimited(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.Header().Set("X-Rate-Limit-Type", "application")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)
	require.Error(t, err)

	var rateLimitErr *RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	assert.Equal(t, 7*time.Second, rateLimitErr.RetryAfter)
	assert.Equal(t, "application", rateLimitErr.Scope)
	assert.Contains(t, rateLimitErr.Endpoint, "/lol/summoner/v4/")

	retryAfter, ok := RetryAfter(err)
	assert.True(t, ok)
	assert.Equal(t, 7*time.Second, retryAfter)
	assert.True(t, IsRetryable(err))
}

// Если Riot не прислал Retry-After, паузу всё равно надо выдержать.
func TestRateLimitedWithoutRetryAfterHeader(t *testing.T) {
	for _, header := range []string{"", "не число", "0", "-3"} {
		t.Run("header="+header, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				if header != "" {
					w.Header().Set("Retry-After", header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			})

			_, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)

			var rateLimitErr *RateLimitError
			require.ErrorAs(t, err, &rateLimitErr)
			assert.Equal(t, defaultRetryAfter, rateLimitErr.RetryAfter)
		})
	}
}

func TestUpstreamError(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "upstream boom")
			})

			_, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)

			var upstreamErr *UpstreamError
			require.ErrorAs(t, err, &upstreamErr)
			assert.Equal(t, status, upstreamErr.StatusCode)
			assert.Equal(t, "upstream boom", upstreamErr.Body)
			assert.True(t, IsRetryable(err), "5xx имеет смысл повторить")
		})
	}
}

func TestUnexpectedClientStatusIsNotRetryable(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	_, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)

	var upstreamErr *UpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	assert.Equal(t, http.StatusBadRequest, upstreamErr.StatusCode)
	assert.False(t, IsRetryable(err))
}

func TestMalformedJSON(t *testing.T) {
	client := newTestClient(t, jsonHandler(t, `{"puuid": `, nil))

	_, err := client.GetAccountByRiotID(context.Background(), "euw1", "Name", "TAG")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "разобрать")
}

func TestUnknownRegionShortCircuits(t *testing.T) {
	var calls int
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, "{}")
	})

	ctx := context.Background()

	_, err := client.GetAccountByRiotID(ctx, "europe", "Name", "TAG")
	assert.ErrorIs(t, err, ErrUnknownRegion)

	_, err = client.GetSummonerByPUUID(ctx, "nope", testPUUID)
	assert.ErrorIs(t, err, ErrUnknownRegion)

	_, err = client.GetLeagueEntriesByPUUID(ctx, "nope", testPUUID)
	assert.ErrorIs(t, err, ErrUnknownRegion)

	_, err = client.GetMatchIDsByPUUID(ctx, "nope", testPUUID, 0, 20)
	assert.ErrorIs(t, err, ErrUnknownRegion)

	_, err = client.GetMatch(ctx, "nope", "EUW1_1")
	assert.ErrorIs(t, err, ErrUnknownRegion)

	assert.Zero(t, calls, "запрос к Riot не должен уходить при неизвестном регионе")
}

func TestContextCancellation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetSummonerByPUUID(ctx, "ru", testPUUID)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// recordingTransport перехватывает запросы, не выпуская их в сеть, — так можно
// проверить реальный хост Riot, а не подменённый через WithBaseURL.
type recordingTransport struct {
	mu       sync.Mutex
	requests []*http.Request
	body     string
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.requests = append(t.requests, req)
	t.mu.Unlock()

	body := t.body
	if body == "" {
		body = "{}"
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

// Главная проверка риска из SPEC.md 7: каждый эндпоинт должен уходить на свой
// вид роутинга. Summoner/League — platform-хост, Account/Match — regional.
func TestEndpointsUseCorrectRoutingHosts(t *testing.T) {
	cases := []struct {
		name     string
		region   string
		call     func(*Client) error
		wantHost string
	}{
		{
			name:   "account → regional",
			region: "ru",
			call: func(c *Client) error {
				_, err := c.GetAccountByRiotID(context.Background(), "ru", "Name", "TAG")

				return err
			},
			wantHost: "europe.api.riotgames.com",
		},
		{
			name:   "summoner → platform",
			region: "ru",
			call: func(c *Client) error {
				_, err := c.GetSummonerByPUUID(context.Background(), "ru", testPUUID)

				return err
			},
			wantHost: "ru.api.riotgames.com",
		},
		{
			name:   "league → platform",
			region: "euw1",
			call: func(c *Client) error {
				_, err := c.GetLeagueEntriesByPUUID(context.Background(), "euw1", testPUUID)

				return err
			},
			wantHost: "euw1.api.riotgames.com",
		},
		{
			name:   "match ids → regional",
			region: "euw1",
			call: func(c *Client) error {
				_, err := c.GetMatchIDsByPUUID(context.Background(), "euw1", testPUUID, 0, 20)

				return err
			},
			wantHost: "europe.api.riotgames.com",
		},
		{
			name:   "match → regional (sea)",
			region: "oc1",
			call: func(c *Client) error {
				_, err := c.GetMatch(context.Background(), "oc1", "OC1_1")

				return err
			},
			wantHost: "sea.api.riotgames.com",
		},
		{
			name:   "account на sea-платформе → asia",
			region: "oc1",
			call: func(c *Client) error {
				_, err := c.GetAccountByRiotID(context.Background(), "oc1", "Name", "TAG")

				return err
			},
			wantHost: "asia.api.riotgames.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &recordingTransport{body: "[]"}
			client := New(testAPIKey, WithHTTPClient(&http.Client{Transport: transport}))

			// Ответ "[]" не подходит объектным DTO — ошибка разбора здесь ожидаема
			// и не мешает проверить адрес запроса.
			_ = tc.call(client)

			require.Len(t, transport.requests, 1)
			req := transport.requests[0]
			assert.Equal(t, "https", req.URL.Scheme)
			assert.Equal(t, tc.wantHost, req.URL.Host)
		})
	}
}
