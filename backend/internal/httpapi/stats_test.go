package httpapi

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
)

func statsDeps(participations ...domain.MatchParticipant) Deps {
	deps := testDeps()
	matches := newFakeMatches()
	matches.participations[testPUUID] = participations
	deps.Matches = matches

	return deps
}

func participation(champion string, kills, deaths, assists int, win bool) domain.MatchParticipant {
	return domain.MatchParticipant{
		PUUID:        testPUUID,
		ChampionName: champion,
		Kills:        kills,
		Deaths:       deaths,
		Assists:      assists,
		Win:          win,
	}
}

func TestGetStatsAggregatesPeriod(t *testing.T) {
	deps := statsDeps(
		participation("Ahri", 10, 2, 5, true),
		participation("Ahri", 6, 4, 2, false),
		participation("Zed", 8, 1, 3, true),
		participation("Zed", 4, 6, 1, true),
	)

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats", "")
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[StatsResponse](t, rec)
	assert.Equal(t, 4, body.Games)
	assert.Equal(t, 3, body.Wins)
	assert.Equal(t, 1, body.Losses)
	assert.InDelta(t, 0.75, body.WinRate, 0.0001)
	// (28 + 11) / 13
	assert.InDelta(t, 39.0/13.0, body.KDA, 0.0001)

	require.Len(t, body.TopChampions, 2)
	assert.Equal(t, "Ahri", body.TopChampions[0].ChampionName)
	assert.Equal(t, 2, body.TopChampions[0].Games)
}

func TestGetStatsDefaultPeriod(t *testing.T) {
	deps := statsDeps()
	matches := deps.Matches.(*fakeMatches)

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats", "")
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody[StatsResponse](t, rec)
	assert.Equal(t, defaultPeriodDays, body.PeriodDays)
	assert.Equal(t, testNow.AddDate(0, 0, -defaultPeriodDays), body.Since)
	assert.Equal(t, body.Since, matches.since, "в хранилище уходит та же граница, что в ответе")
}

func TestGetStatsHonoursPeriodParameter(t *testing.T) {
	deps := statsDeps()
	matches := deps.Matches.(*fakeMatches)

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats?period=7d", "")
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody[StatsResponse](t, rec)
	assert.Equal(t, 7, body.PeriodDays)
	assert.Equal(t, testNow.AddDate(0, 0, -7), matches.since)
}

func TestGetStatsEmptyPeriodIsNotAnError(t *testing.T) {
	rec := call(t, statsDeps(), http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats?period=1d", "")
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[StatsResponse](t, rec)
	assert.Zero(t, body.Games)
	assert.Zero(t, body.WinRate)
	assert.Zero(t, body.KDA)

	// Пустой массив, а не null.
	assert.Contains(t, rec.Body.String(), `"topChampions":[]`)
}

func TestGetStatsRejectsBadPeriod(t *testing.T) {
	for _, period := range []string{
		"0d",
		"-1d",
		strconv.Itoa(maxPeriodDays+1) + "d",
		"abc",
		"30",
		"30days",
		"30h",
		"d",
	} {
		t.Run(period, func(t *testing.T) {
			deps := statsDeps()
			matches := deps.Matches.(*fakeMatches)

			rec := call(t, deps, http.MethodGet,
				"/api/v1/summoners/"+testPUUID+"/stats?period="+period, "")
			requireErrorCode(t, rec, http.StatusBadRequest, "invalid_period")

			assert.True(t, matches.since.IsZero(), "в хранилище ходить не надо")
		})
	}
}

func TestGetStatsAcceptsBoundaryPeriods(t *testing.T) {
	for _, days := range []int{1, maxPeriodDays} {
		rec := call(t, statsDeps(), http.MethodGet,
			"/api/v1/summoners/"+testPUUID+"/stats?period="+strconv.Itoa(days)+"d", "")
		require.Equal(t, http.StatusOK, rec.Code, "period=%dd должен приниматься", days)

		assert.Equal(t, days, decodeBody[StatsResponse](t, rec).PeriodDays)
	}
}

func TestGetStatsNormalizesPeriod(t *testing.T) {
	rec := call(t, statsDeps(), http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats?period=%2014D%20", "")
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	assert.Equal(t, 14, decodeBody[StatsResponse](t, rec).PeriodDays)
}

func TestGetStatsReturns404ForUnknownSummoner(t *testing.T) {
	rec := call(t, statsDeps(), http.MethodGet, "/api/v1/summoners/нет-такого/stats", "")
	requireErrorCode(t, rec, http.StatusNotFound, "summoner_not_found")
}

func TestGetStatsReturns500OnStorageFailure(t *testing.T) {
	deps := testDeps()
	matches := newFakeMatches()
	matches.err = errDatabaseDown
	deps.Matches = matches

	rec := call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats", "")
	requireErrorCode(t, rec, http.StatusInternalServerError, "internal_error")
}

func TestGetStatsDoesNotCallRiot(t *testing.T) {
	deps := statsDeps(participation("Ahri", 1, 1, 1, true))
	profiles := deps.Profiles.(*fakeProfiles)

	require.Equal(t, http.StatusOK,
		call(t, deps, http.MethodGet, "/api/v1/summoners/"+testPUUID+"/stats", "").Code)
	assert.Zero(t, profiles.calls)
}
