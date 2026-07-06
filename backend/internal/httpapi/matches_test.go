package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawMatch — фрагмент ответа Match-V5 в том виде, в каком он лежит в matches.raw_data.
// Поля вроде item0 и summoner1Id в наши DTO не попадают — и именно поэтому эндпоинт
// отдаёт сырой JSON (CLAUDE.md, отклонение 1).
const rawMatch = `{"metadata":{"matchId":"EUW1_7","participants":["p1","p2"]},` +
	`"info":{"gameDuration":1500,"participants":[{"puuid":"p1","championName":"Ahri","item0":3157},` +
	`{"puuid":"p2","championName":"Zed","summoner1Id":4}]}}`

func testRawDeps() Deps {
	deps := testDeps()
	deps.Matches = &fakeMatches{raw: map[string]json.RawMessage{"EUW1_7": json.RawMessage(rawMatch)}}

	return deps
}

// Ответ Riot доходит до клиента байт в байт: пересериализация могла бы потерять
// поля, которых нет в наших структурах.
func TestGetMatchReturnsRawJSONVerbatim(t *testing.T) {
	rec := call(t, testRawDeps(), http.MethodGet, "/api/v1/matches/EUW1_7", "")

	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.JSONEq(t, rawMatch, rec.Body.String())

	// Поля, которых нет в MatchListItemResponse, обязаны дойти как есть.
	assert.Contains(t, rec.Body.String(), `"item0":3157`)
	assert.Contains(t, rec.Body.String(), `"summoner1Id":4`)
}

// Обе команды и все десять участников — из raw_data, а не из match_participants,
// где лежат только отслеживаемые саммонеры.
func TestGetMatchReturnsAllParticipants(t *testing.T) {
	full := map[string]any{
		"metadata": map[string]any{"matchId": "EUW1_10"},
		"info":     map[string]any{"participants": make([]any, 0, 10)},
	}

	participants := make([]any, 0, 10)
	for i := range 10 {
		participants = append(participants, map[string]any{
			"puuid":  "p" + string(rune('0'+i)),
			"teamId": 100 + (i/5)*100,
		})
	}
	full["info"] = map[string]any{"participants": participants}

	encoded, err := json.Marshal(full)
	require.NoError(t, err)

	deps := testDeps()
	deps.Matches = &fakeMatches{raw: map[string]json.RawMessage{"EUW1_10": encoded}}

	rec := call(t, deps, http.MethodGet, "/api/v1/matches/EUW1_10", "")
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody[struct {
		Info struct {
			Participants []struct {
				PUUID  string `json:"puuid"`
				TeamID int    `json:"teamId"`
			} `json:"participants"`
		} `json:"info"`
	}](t, rec)

	require.Len(t, body.Info.Participants, 10)
	assert.Equal(t, 100, body.Info.Participants[0].TeamID)
	assert.Equal(t, 200, body.Info.Participants[9].TeamID)
}

func TestGetMatchReturns404ForUnknownID(t *testing.T) {
	rec := call(t, testRawDeps(), http.MethodGet, "/api/v1/matches/EUW1_нет-такого", "")
	requireErrorCode(t, rec, http.StatusNotFound, "match_not_found")
}

func TestGetMatchReturns500OnStorageFailure(t *testing.T) {
	deps := testDeps()
	deps.Matches = &fakeMatches{err: errDatabaseDown}

	rec := call(t, deps, http.MethodGet, "/api/v1/matches/EUW1_7", "")
	requireErrorCode(t, rec, http.StatusInternalServerError, "internal_error")
}
