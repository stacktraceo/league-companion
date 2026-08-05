package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

const testPUUID = "test-puuid-0000"

func TestSummonerFromRiot(t *testing.T) {
	account := riot.AccountDTO{
		PUUID:    testPUUID,
		GameName: "Test Summoner",
		TagLine:  "EUW",
	}
	summoner := riot.SummonerDTO{
		PUUID:         testPUUID,
		ProfileIconID: 5678,
		SummonerLevel: 412,
	}

	got := SummonerFromRiot(account, summoner, "euw1")

	assert.Equal(t, testPUUID, got.PUUID)
	assert.Equal(t, "Test Summoner", got.RiotID)
	assert.Equal(t, "EUW", got.TagLine)
	assert.Equal(t, "euw1", got.Region, "регион берётся от вызывающего, Riot его не отдаёт")
	assert.Equal(t, 412, got.SummonerLevel)
	assert.Equal(t, 5678, got.ProfileIconID)
	assert.Nil(t, got.LastSyncedAt, "свежий саммонер ещё не синхронизировался")
	assert.Equal(t, "Test Summoner#EUW", got.FullRiotID())
}

func TestRankedStatsFromRiot(t *testing.T) {
	updatedAt := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	entries := []riot.LeagueEntryDTO{
		{QueueType: "RANKED_SOLO_5x5", Tier: "GOLD", Rank: "II", LeaguePoints: 47, Wins: 63, Losses: 58},
		{QueueType: "RANKED_FLEX_SR", Tier: "SILVER", Rank: "I", LeaguePoints: 12, Wins: 9, Losses: 11},
	}

	stats := RankedStatsFromRiot(testPUUID, entries, updatedAt)
	require.Len(t, stats, 2)

	assert.Equal(t, RankedStat{
		PUUID:        testPUUID,
		QueueType:    "RANKED_SOLO_5x5",
		Tier:         "GOLD",
		Rank:         "II",
		LeaguePoints: 47,
		Wins:         63,
		Losses:       58,
		UpdatedAt:    updatedAt,
	}, stats[0])

	assert.Equal(t, "RANKED_FLEX_SR", stats[1].QueueType)
	assert.Equal(t, updatedAt, stats[1].UpdatedAt)
}

func TestRankedStatsFromRiotUnranked(t *testing.T) {
	assert.Nil(t, RankedStatsFromRiot(testPUUID, nil, time.Now()))
	assert.Nil(t, RankedStatsFromRiot(testPUUID, []riot.LeagueEntryDTO{}, time.Now()))
}

func TestMatchFromRiot(t *testing.T) {
	raw := json.RawMessage(`{"metadata":{"matchId":"EUW1_7000000001"},"extra":"kept"}`)
	detail := riot.MatchDetail{
		Match: riot.MatchDTO{
			Metadata: riot.MatchMetadataDTO{MatchID: "EUW1_7000000001"},
			Info: riot.MatchInfoDTO{
				GameCreation:     1700000000000,
				GameDuration:     1834,
				GameEndTimestamp: 1700001834000,
				GameVersion:      "14.1.556.1234",
				QueueID:          420,
			},
		},
		Raw: raw,
	}

	match, err := MatchFromRiot(detail)
	require.NoError(t, err)

	assert.Equal(t, "EUW1_7000000001", match.MatchID)
	assert.Equal(t, time.UnixMilli(1700000000000).UTC(), match.GameCreation)
	assert.Equal(t, 1834*time.Second, match.GameDuration)
	assert.Equal(t, 1834, match.DurationSeconds())
	assert.Equal(t, 420, match.QueueID)
	assert.Equal(t, "14.1.556.1234", match.GameVersion)
	assert.JSONEq(t, string(raw), string(match.RawData), "сырой JSON должен доехать до raw_data без изменений")
}

func TestMatchDurationHandlesPre1120Millis(t *testing.T) {
	cases := []struct {
		name             string
		gameDuration     int64
		gameEndTimestamp int64
		want             time.Duration
	}{
		{"новый матч: секунды", 1834, 1700001834000, 1834 * time.Second},
		{"старый матч: миллисекунды", 1834000, 0, 1834 * time.Second},
		{"нулевая длительность", 0, 1700001834000, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail := riot.MatchDetail{
				Match: riot.MatchDTO{
					Metadata: riot.MatchMetadataDTO{MatchID: "EUW1_1"},
					Info: riot.MatchInfoDTO{
						GameDuration:     tc.gameDuration,
						GameEndTimestamp: tc.gameEndTimestamp,
					},
				},
				Raw: json.RawMessage(`{}`),
			}

			match, err := MatchFromRiot(detail)
			require.NoError(t, err)
			assert.Equal(t, tc.want, match.GameDuration)
		})
	}
}

func TestMatchFromRiotRequiresMatchID(t *testing.T) {
	_, err := MatchFromRiot(riot.MatchDetail{Raw: json.RawMessage(`{}`)})
	assert.ErrorIs(t, err, ErrEmptyMatch)

	_, err = MatchParticipantsFromRiot(riot.MatchDetail{Raw: json.RawMessage(`{}`)})
	assert.ErrorIs(t, err, ErrEmptyMatch)
}

func TestMatchParticipantsFromRiot(t *testing.T) {
	detail := riot.MatchDetail{
		Match: riot.MatchDTO{
			Metadata: riot.MatchMetadataDTO{MatchID: "EUW1_7000000001"},
			Info: riot.MatchInfoDTO{
				Participants: []riot.ParticipantDTO{
					{
						PUUID:                testPUUID,
						ChampionName:         "Ahri",
						Kills:                11,
						Deaths:               3,
						Assists:              9,
						Win:                  true,
						GoldEarned:           14320,
						TotalMinionsKilled:   187,
						NeutralMinionsKilled: 14,
					},
					{
						PUUID:                "other-puuid",
						ChampionName:         "Darius",
						Kills:                4,
						Deaths:               8,
						Assists:              2,
						Win:                  false,
						GoldEarned:           10110,
						TotalMinionsKilled:   154,
						NeutralMinionsKilled: 0,
					},
				},
			},
		},
		Raw: json.RawMessage(`{}`),
	}

	participants, err := MatchParticipantsFromRiot(detail)
	require.NoError(t, err)
	require.Len(t, participants, 2, "маппер отдаёт всех участников, фильтрация - на вызывающем")

	assert.Equal(t, MatchParticipant{
		MatchID:      "EUW1_7000000001",
		PUUID:        testPUUID,
		ChampionName: "Ahri",
		Kills:        11,
		Deaths:       3,
		Assists:      9,
		Win:          true,
		CS:           201,
		GoldEarned:   14320,
	}, participants[0])

	assert.Equal(t, 154, participants[1].CS, "CS = миньоны + лесные монстры")
	assert.False(t, participants[1].Win)
}

func TestMatchParticipantsEmpty(t *testing.T) {
	detail := riot.MatchDetail{
		Match: riot.MatchDTO{Metadata: riot.MatchMetadataDTO{MatchID: "EUW1_1"}},
		Raw:   json.RawMessage(`{}`),
	}

	participants, err := MatchParticipantsFromRiot(detail)
	require.NoError(t, err)
	assert.Empty(t, participants)
}

func TestKDA(t *testing.T) {
	cases := []struct {
		name string
		p    MatchParticipant
		want float64
	}{
		{"обычный", MatchParticipant{Kills: 10, Deaths: 5, Assists: 5}, 3},
		{"без смертей", MatchParticipant{Kills: 7, Deaths: 0, Assists: 3}, 10},
		{"пустой", MatchParticipant{}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, tc.p.KDA(), 0.0001)
		})
	}
}
