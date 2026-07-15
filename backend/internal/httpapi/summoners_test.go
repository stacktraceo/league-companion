package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
	"github.com/stacktraceo/league-companion/backend/internal/syncer"
)

const validCreateBody = `{"riotId":"Test Summoner","tagLine":"EUW","region":"euw1"}`

func TestCreateSummonerReturns201AndProfile(t *testing.T) {
	deps := testDeps()
	deps.Ranked = &fakeRanked{items: map[string][]domain.RankedStat{
		testPUUID: {{
			QueueType:    "RANKED_SOLO_5x5",
			Tier:         "GOLD",
			Rank:         "II",
			LeaguePoints: 47,
			Wins:         120,
			Losses:       98,
			UpdatedAt:    testCreatedAt,
		}},
	}}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	require.Equal(t, http.StatusCreated, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[SummonerResponse](t, rec)
	assert.Equal(t, testPUUID, body.PUUID)
	assert.Equal(t, "Test Summoner", body.RiotID)
	assert.Equal(t, 412, body.SummonerLevel)
	assert.False(t, body.Stale)

	require.Len(t, body.Ranked, 1)
	assert.Equal(t, "RANKED_SOLO_5x5", body.Ranked[0].QueueType)
	assert.Equal(t, 47, body.Ranked[0].LeaguePoints)
}

// Повторное добавление уже известного саммонера — не ошибка и не 201: профиль
// обновляется, статус 200.
func TestCreateSummonerReturns200WhenAlreadyTracked(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{summoner: testSummoner(), created: false}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	assert.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
}

// Матчи должны уходить в фон: держать HTTP-соединение ради двух десятков запросов
// к Riot незачем (решение 1 плана вехи).
func TestCreateSummonerEnqueuesMatchSync(t *testing.T) {
	deps := testDeps()
	queue := deps.Queue.(*fakeQueue)

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	require.Equal(t, http.StatusCreated, rec.Code)

	assert.Equal(t, []string{testPUUID}, queue.enqueued)
}

// Переполненная очередь не должна ронять запрос: профиль уже сохранён, матчи
// подтянет следующий прогон.
func TestCreateSummonerSurvivesFullQueue(t *testing.T) {
	deps := testDeps()
	deps.Queue = &fakeQueue{reject: true}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	assert.Equal(t, http.StatusCreated, rec.Code, "тело: %s", rec.Body.String())
}

// Ранги — не повод терять уже сохранённый профиль.
func TestCreateSummonerSurvivesRankedReadFailure(t *testing.T) {
	deps := testDeps()
	deps.Ranked = &fakeRanked{err: errDatabaseDown}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	require.Equal(t, http.StatusCreated, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[SummonerResponse](t, rec)
	assert.Equal(t, testPUUID, body.PUUID)
	assert.Empty(t, body.Ranked)
}

// Регион приводится к нижнему регистру, пробелы срезаются, лишний «#» в теге
// прощается — клиенту незачем знать наши внутренние соглашения.
func TestCreateSummonerNormalizesInput(t *testing.T) {
	deps := testDeps()
	profiles := deps.Profiles.(*fakeProfiles)

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners",
		`{"riotId":"  Test Summoner  ","tagLine":"#euw","region":" EUW1 "}`)
	require.Equal(t, http.StatusCreated, rec.Code, "тело: %s", rec.Body.String())

	assert.Equal(t, "euw1", profiles.region)
	assert.Equal(t, "Test Summoner", profiles.gameName)
	assert.Equal(t, "euw", profiles.tagLine)
}

func TestCreateSummonerRejectsInvalidBody(t *testing.T) {
	tests := map[string]struct {
		body string
		code string
	}{
		"не json":            {body: `{`, code: "invalid_body"},
		"не объект":          {body: `[]`, code: "invalid_body"},
		"пустое тело":        {body: ``, code: "invalid_body"},
		"лишнее поле":        {body: `{"riotId":"A","tagLine":"EUW","region":"euw1","puuid":"x"}`, code: "invalid_body"},
		"нет riotId":         {body: `{"tagLine":"EUW","region":"euw1"}`, code: "invalid_request"},
		"тег внутри riotId":  {body: `{"riotId":"A#EUW","tagLine":"EUW","region":"euw1"}`, code: "invalid_request"},
		"нет tagLine":        {body: `{"riotId":"A","region":"euw1"}`, code: "invalid_request"},
		"нет региона":        {body: `{"riotId":"A","tagLine":"EUW"}`, code: "invalid_request"},
		"неизвестный регион": {body: `{"riotId":"A","tagLine":"EUW","region":"мордор"}`, code: "invalid_request"},
		// «europe» — regional-маршрут, не платформа; перепутать их легко (SPEC.md 7),
		// поэтому регион проверяется до похода в Riot.
		"regional вместо платформы": {body: `{"riotId":"A","tagLine":"EUW","region":"europe"}`, code: "invalid_request"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			deps := testDeps()
			profiles := deps.Profiles.(*fakeProfiles)

			rec := call(t, deps, http.MethodPost, "/api/v1/summoners", test.body)
			requireErrorCode(t, rec, http.StatusBadRequest, test.code)

			assert.Zero(t, profiles.calls, "в Riot ходить не надо — запрос невалиден")
		})
	}
}

// Тело больше maxRequestBody не должно уходить в парсер целиком.
func TestCreateSummonerRejectsOversizedBody(t *testing.T) {
	deps := testDeps()
	huge := `{"riotId":"` + strings.Repeat("a", maxRequestBody+1) + `","tagLine":"EUW","region":"euw1"}`

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", huge)
	requireErrorCode(t, rec, http.StatusBadRequest, "invalid_body")
}

// Маппинг ошибок Riot на коды API (SPEC.md 3.4).
func TestCreateSummonerMapsUpstreamErrors(t *testing.T) {
	tests := map[string]struct {
		err    error
		status int
		code   string
	}{
		"саммонер не найден": {err: riot.ErrNotFound, status: http.StatusNotFound, code: "summoner_not_found"},
		"неизвестный регион": {err: riot.ErrUnknownRegion, status: http.StatusBadRequest, code: "invalid_region"},
		// Отклонён наш ключ, а не клиентский, — это 502, а не 401.
		"протухший ключ":  {err: riot.ErrUnauthorized, status: http.StatusBadGateway, code: "riot_unauthorized"},
		"таймаут":         {err: context.DeadlineExceeded, status: http.StatusGatewayTimeout, code: "riot_timeout"},
		"запрос отменён":  {err: context.Canceled, status: http.StatusGatewayTimeout, code: "riot_timeout"},
		"Riot прилёг":     {err: &riot.UpstreamError{StatusCode: 503}, status: http.StatusBadGateway, code: "riot_unavailable"},
		"прочая ошибка":   {err: errors.New("что-то не то"), status: http.StatusBadGateway, code: "riot_unavailable"},
		"обёрнутая 404":   {err: fmt.Errorf("резолв Riot ID: %w", riot.ErrNotFound), status: http.StatusNotFound, code: "summoner_not_found"},
		"нет в хранилище": {err: storage.ErrNotFound, status: http.StatusNotFound, code: "summoner_not_found"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			deps := testDeps()
			deps.Profiles = &fakeProfiles{err: test.err}
			// Хранилище пустое намеренно: при сохранённом снапшоте случаи
			// «до Riot не дозвонились» отвечают 200 со stale, и тогда этот тест
			// проверял бы не маппинг ошибок, а обход вокруг него.
			deps.Summoners = &fakeSummoners{items: map[string]domain.Summoner{}}

			rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
			requireErrorCode(t, rec, test.status, test.code)
		})
	}
}

// Главный сценарий SPEC.md 3.4: Riot лежит (чаще всего — протух 24-часовой ключ),
// но саммонер уже отслеживается, и отдать есть что.
func TestCreateSummonerServesCachedProfileWhenRiotIsDown(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{err: riot.ErrUnauthorized}
	deps.Ranked = &fakeRanked{items: map[string][]domain.RankedStat{
		testPUUID: {{QueueType: "RANKED_SOLO_5x5", Tier: "GOLD", Rank: "II", LeaguePoints: 47}},
	}}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[SummonerResponse](t, rec)
	assert.True(t, body.Stale)
	assert.Equal(t, testPUUID, body.PUUID)
	assert.Equal(t, 412, body.SummonerLevel)
	// Ранги идут вместе с профилем: снапшот отдаётся целиком, а не наполовину.
	require.Len(t, body.Ranked, 1)
	assert.Equal(t, 47, body.Ranked[0].LeaguePoints)
}

// Riot только что не ответил — ставить в очередь задачу, которая гарантированно
// упадёт в фоне, незачем.
func TestCreateSummonerDoesNotEnqueueOnStale(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{err: riot.ErrUnauthorized}
	queue := deps.Queue.(*fakeQueue)

	require.Equal(t, http.StatusOK, call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody).Code)
	assert.Empty(t, queue.enqueued)
}

// Незнакомый саммонер при лежащем Riot — обычная ошибка: отдавать нечего, и 502
// честно значит «запрос не выполнен».
func TestCreateSummonerReturns502WhenNothingCached(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{err: riot.ErrUnauthorized}
	deps.Summoners = &fakeSummoners{items: map[string]domain.Summoner{}}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	requireErrorCode(t, rec, http.StatusBadGateway, "riot_unauthorized")

	assert.NotContains(t, rec.Body.String(), "puuid", "в ответе с ошибкой данных быть не должно")
}

// Не всякая ошибка — повод показать старое. 404 и 400 значат, что отдавать нечего по
// существу, а 429 просит подождать названный Riot срок, а не смотреть вчерашние цифры.
func TestCreateSummonerDoesNotServeStaleForClientErrors(t *testing.T) {
	tests := map[string]struct {
		err    error
		status int
		code   string
	}{
		"саммонер не найден": {err: riot.ErrNotFound, status: http.StatusNotFound, code: "summoner_not_found"},
		"неизвестный регион": {err: riot.ErrUnknownRegion, status: http.StatusBadRequest, code: "invalid_region"},
		"лимит Riot": {
			err:    &riot.RateLimitError{RetryAfter: 7 * time.Second, Scope: "application"},
			status: http.StatusTooManyRequests,
			code:   "rate_limited",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Снапшот в хранилище есть — и всё равно не отдаётся.
			deps := testDeps()
			deps.Profiles = &fakeProfiles{err: test.err}

			rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
			requireErrorCode(t, rec, test.status, test.code)
		})
	}
}

// Регистр введённого имени не должен мешать найти снапшот: пользователь набирает ник
// руками, а в базе лежит написание от Riot.
func TestCreateSummonerFindsCachedProfileIgnoringCase(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{err: riot.ErrUnauthorized}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners",
		`{"riotId":"test summoner","tagLine":"euw","region":"EUW1"}`)
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	assert.True(t, decodeBody[SummonerResponse](t, rec).Stale)
}

// На 429 отдаём наружу тот же срок, что назвал Riot: клиенту нужно знать, сколько ждать.
func TestCreateSummonerPassesRetryAfterOn429(t *testing.T) {
	deps := testDeps()
	deps.Profiles = &fakeProfiles{err: &riot.RateLimitError{RetryAfter: 7 * time.Second, Scope: "application"}}

	rec := call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody)
	requireErrorCode(t, rec, http.StatusTooManyRequests, "rate_limited")

	assert.Equal(t, "7", rec.Header().Get("Retry-After"))
}

// Хендлер просит у очереди ровно столько матчей, сколько тянет фоновая синхронизация.
func TestCreateSummonerUsesDefaultMatchCount(t *testing.T) {
	deps := testDeps()
	queue := &countingQueue{}
	deps.Queue = queue

	require.Equal(t, http.StatusCreated, call(t, deps, http.MethodPost, "/api/v1/summoners", validCreateBody).Code)
	assert.Equal(t, syncer.DefaultMatchCount, queue.count)
}

func TestGetSummonerReturnsProfileFromStorage(t *testing.T) {
	deps := testDeps()

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID, "")
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[SummonerResponse](t, rec)
	assert.Equal(t, testPUUID, body.PUUID)
	assert.Equal(t, "euw1", body.Region)

	// Пустой список, а не null: клиенту без ранга проще перебирать пустой массив.
	assert.NotNil(t, body.Ranked)
	assert.Empty(t, body.Ranked)
}

// GET не должен ходить в Riot вовсе — иначе в 200 мс (SPEC.md 3.6) не уложиться.
func TestGetSummonerDoesNotCallRiot(t *testing.T) {
	deps := testDeps()
	profiles := deps.Profiles.(*fakeProfiles)

	require.Equal(t, http.StatusOK, call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID, "").Code)
	assert.Zero(t, profiles.calls)
}

func TestGetSummonerReturns404ForUnknownPUUID(t *testing.T) {
	rec := call(t, testDeps(), http.MethodGet, "/api/v1/summoners/нет-такого", "")
	requireErrorCode(t, rec, http.StatusNotFound, "summoner_not_found")
}

func TestGetSummonerReturns500OnStorageFailure(t *testing.T) {
	deps := testDeps()
	deps.Summoners = &fakeSummoners{err: errDatabaseDown}

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID, "")
	body := requireErrorCode(t, rec, http.StatusInternalServerError, "internal_error")

	// Детали внутренней ошибки наружу не отдаём.
	assert.NotContains(t, body.Message, "база прилегла")
}

func TestListMatchesReturnsPage(t *testing.T) {
	deps := testDeps()
	deps.Matches = &fakeMatches{items: map[string][]storage.MatchListItem{
		testPUUID: matchItems(3),
	}}

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches", "")
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[MatchListResponse](t, rec)
	require.Len(t, body.Items, 3)
	assert.Equal(t, defaultLimit, body.Limit)
	assert.Equal(t, 0, body.Offset)
	assert.Equal(t, 3, body.Total)

	first := body.Items[0]
	assert.Equal(t, "EUW1_0", first.MatchID)
	assert.Equal(t, "Ahri", first.ChampionName)
	assert.Equal(t, 1500, first.GameDurationSeconds)
	// KDA считается на стороне API: (11 + 9) / 3.
	assert.InDelta(t, 20.0/3.0, first.KDA, 0.0001)
}

func TestListMatchesPaginates(t *testing.T) {
	deps := testDeps()
	deps.Matches = &fakeMatches{items: map[string][]storage.MatchListItem{
		testPUUID: matchItems(7),
	}}

	firstPage := decodeBody[MatchListResponse](t,
		call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches?limit=5&offset=0", ""))
	require.Len(t, firstPage.Items, 5)
	assert.Equal(t, 5, firstPage.Limit)
	assert.Equal(t, 7, firstPage.Total, "total — всего матчей, а не размер страницы")

	secondPage := decodeBody[MatchListResponse](t,
		call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches?limit=5&offset=5", ""))
	require.Len(t, secondPage.Items, 2)
	assert.Equal(t, 5, secondPage.Offset)

	assert.NotEqual(t, firstPage.Items[0].MatchID, secondPage.Items[0].MatchID)
}

// Пустая страница — это пустой массив, а не null: иначе Android-клиенту пришлось бы
// разбирать оба случая.
func TestListMatchesReturnsEmptyArrayNotNull(t *testing.T) {
	rec := call(t, testDeps(), http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches", "")
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Body.String(), `"items":[]`)
}

// Некорректную пагинацию отвергаем, а не подставляем молча свои числа.
func TestListMatchesRejectsBadPagination(t *testing.T) {
	for _, query := range []string{
		"?limit=0",
		"?limit=-1",
		"?limit=" + strconv.Itoa(maxLimit+1),
		"?limit=много",
		"?limit=1.5",
		"?offset=-1",
		"?offset=назад",
	} {
		t.Run(query, func(t *testing.T) {
			rec := call(t, testDeps(), http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches"+query, "")
			requireErrorCode(t, rec, http.StatusBadRequest, "invalid_pagination")
		})
	}
}

func TestListMatchesAcceptsBoundaryLimits(t *testing.T) {
	deps := testDeps()

	for _, limit := range []int{1, maxLimit} {
		rec := call(t, deps, http.MethodGet,
			"/api/v1/summoners/"+testPUUID+"/matches?limit="+strconv.Itoa(limit), "")
		require.Equal(t, http.StatusOK, rec.Code, "limit=%d должен приниматься", limit)

		assert.Equal(t, limit, decodeBody[MatchListResponse](t, rec).Limit)
	}
}

// «Саммонер не отслеживается» и «матчей пока нет» — разные ситуации для клиента.
func TestListMatchesReturns404ForUnknownSummoner(t *testing.T) {
	deps := testDeps()
	deps.Matches = &fakeMatches{items: map[string][]storage.MatchListItem{}}

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/нет-такого/matches", "")
	requireErrorCode(t, rec, http.StatusNotFound, "summoner_not_found")
}

// Пагинация проверяется до похода в базу: смысла читать матчи при limit=0 нет.
func TestListMatchesValidatesPaginationBeforeStorage(t *testing.T) {
	deps := testDeps()
	matches := &fakeMatches{items: map[string][]storage.MatchListItem{testPUUID: matchItems(3)}}
	deps.Matches = matches

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches?limit=0", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	assert.Zero(t, matches.listCalls, "в хранилище ходить не надо")
}

func TestListMatchesReturns500OnStorageFailure(t *testing.T) {
	deps := testDeps()
	deps.Matches = &fakeMatches{err: errDatabaseDown}

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/matches", "")
	requireErrorCode(t, rec, http.StatusInternalServerError, "internal_error")
}
