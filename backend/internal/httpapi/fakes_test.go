package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
)

const (
	testAPIKey = "тестовый-секрет"
	testPUUID  = "puuid-1"
)

var (
	testCreatedAt = time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)

	// testNow - подменённые «сейчас» для границы периода в /stats и cooldown /sync.
	testNow = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(context.Context) error { return s.err }

type fakeProfiles struct {
	summoner domain.Summoner
	created  bool
	err      error

	region   string
	gameName string
	tagLine  string
	calls    int
}

func (f *fakeProfiles) SyncProfile(
	_ context.Context,
	region, gameName, tagLine string,
) (domain.Summoner, bool, error) {
	f.calls++
	f.region, f.gameName, f.tagLine = region, gameName, tagLine

	if f.err != nil {
		return domain.Summoner{}, false, f.err
	}

	return f.summoner, f.created, nil
}

type fakeQueue struct {
	reject   bool
	enqueued []string
}

func (f *fakeQueue) Enqueue(puuid string, _ int) bool {
	if f.reject {
		return false
	}

	f.enqueued = append(f.enqueued, puuid)

	return true
}

type countingQueue struct {
	count int
	calls int
}

func (q *countingQueue) Enqueue(_ string, count int) bool {
	q.calls++
	q.count = count

	return true
}

type fakeSummoners struct {
	items map[string]domain.Summoner
	err   error
}

func (f *fakeSummoners) ByPUUID(_ context.Context, puuid string) (domain.Summoner, error) {
	if f.err != nil {
		return domain.Summoner{}, f.err
	}

	summoner, ok := f.items[puuid]
	if !ok {
		return domain.Summoner{}, storage.ErrNotFound
	}

	return summoner, nil
}

func (f *fakeSummoners) ByRiotID(_ context.Context, region, gameName, tagLine string) (domain.Summoner, error) {
	if f.err != nil {
		return domain.Summoner{}, f.err
	}

	for _, summoner := range f.items {
		if strings.EqualFold(summoner.Region, region) &&
			strings.EqualFold(summoner.RiotID, gameName) &&
			strings.EqualFold(summoner.TagLine, tagLine) {
			return summoner, nil
		}
	}

	return domain.Summoner{}, storage.ErrNotFound
}

type fakeRanked struct {
	items map[string][]domain.RankedStat
	err   error
}

func (f *fakeRanked) ByPUUID(_ context.Context, puuid string) ([]domain.RankedStat, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.items[puuid], nil
}

type fakeMatches struct {
	items map[string][]storage.MatchListItem
	raw   map[string]json.RawMessage

	// participations - вход агрегации; отдельно от items, потому что тесты
	// статистики задают K/D/A, а не строки ленты.
	participations map[string][]domain.MatchParticipant

	err error

	limit     int
	offset    int
	listCalls int
	since     time.Time
}

func (f *fakeMatches) ParticipationsSince(
	_ context.Context,
	puuid string,
	since time.Time,
) ([]domain.MatchParticipant, error) {
	if f.err != nil {
		return nil, f.err
	}

	f.since = since

	return f.participations[puuid], nil
}

func (f *fakeMatches) ListByPUUID(
	_ context.Context,
	puuid string,
	limit, offset int,
) ([]storage.MatchListItem, error) {
	f.listCalls++

	if f.err != nil {
		return nil, f.err
	}

	f.limit, f.offset = limit, offset

	all := f.items[puuid]
	if offset >= len(all) {
		return nil, nil
	}

	return all[offset:min(offset+limit, len(all))], nil
}

func (f *fakeMatches) CountByPUUID(_ context.Context, puuid string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}

	return len(f.items[puuid]), nil
}

func (f *fakeMatches) RawByID(_ context.Context, matchID string) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}

	raw, ok := f.raw[matchID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return raw, nil
}

func testSummoner() domain.Summoner {
	return domain.Summoner{
		PUUID:         testPUUID,
		RiotID:        "Test Summoner",
		TagLine:       "EUW",
		Region:        "euw1",
		SummonerLevel: 412,
		ProfileIconID: 5678,
		CreatedAt:     testCreatedAt,
	}
}

func testMatchItem(matchID string, creation time.Time) storage.MatchListItem {
	return storage.MatchListItem{
		MatchID:      matchID,
		GameCreation: creation,
		GameDuration: 25 * time.Minute,
		QueueID:      420,
		GameVersion:  "14.1.556.1234",
		ChampionName: "Ahri",
		Kills:        11,
		Deaths:       3,
		Assists:      9,
		Win:          true,
		CS:           201,
		GoldEarned:   14320,
	}
}

func newFakeMatches() *fakeMatches {
	return &fakeMatches{
		items:          map[string][]storage.MatchListItem{},
		raw:            map[string]json.RawMessage{},
		participations: map[string][]domain.MatchParticipant{},
	}
}

func matchItems(n int) []storage.MatchListItem {
	items := make([]storage.MatchListItem, 0, n)

	for i := range n {
		created := testCreatedAt.Add(-time.Duration(i) * time.Hour)
		items = append(items, testMatchItem("EUW1_"+strconv.Itoa(i), created))
	}

	return items
}

func testDeps() Deps {
	return Deps{
		Logger:       testLogger(),
		DB:           stubPinger{},
		ClientAPIKey: testAPIKey,
		Profiles:     &fakeProfiles{summoner: testSummoner(), created: true},
		Queue:        &fakeQueue{},
		Summoners:    &fakeSummoners{items: map[string]domain.Summoner{testPUUID: testSummoner()}},
		Ranked:       &fakeRanked{items: map[string][]domain.RankedStat{}},
		Matches:      newFakeMatches(),
		Now:          func() time.Time { return testNow },
	}
}

func newTestRouter(t *testing.T, deps Deps) *chi.Mux {
	t.Helper()

	return NewRouter(deps)
}

func newHealthRouter(t *testing.T, db Pinger) *chi.Mux {
	t.Helper()

	deps := testDeps()
	deps.DB = db

	return NewRouter(deps)
}

func authRequest(method, path, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequest(method, path, reader)
	request.Header.Set(apiKeyHeader, testAPIKey)

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	return request
}

func call(t *testing.T, deps Deps, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	newTestRouter(t, deps).ServeHTTP(rec, authRequest(method, path, body))

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var target T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &target), "тело: %s", rec.Body.String())

	return target
}

func requireErrorCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) ErrorResponse {
	t.Helper()

	require.Equal(t, status, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[ErrorResponse](t, rec)
	assert.Equal(t, code, body.Error)
	assert.NotEmpty(t, body.Message, "сообщение об ошибке должно быть человекочитаемым")

	return body
}

var errDatabaseDown = errors.New("база прилегла")
