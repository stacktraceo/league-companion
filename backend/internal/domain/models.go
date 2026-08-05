package domain

import (
	"encoding/json"
	"time"
)

type Summoner struct {
	PUUID         string
	RiotID        string // GameName из Riot ID
	TagLine       string
	Region        string // platform routing: ru, euw1, ...
	SummonerLevel int
	ProfileIconID int
	LastSyncedAt  *time.Time // nil, пока ни разу не синхронизировался
	CreatedAt     time.Time
}

func (s Summoner) FullRiotID() string {
	return s.RiotID + "#" + s.TagLine
}

type RankedStat struct {
	PUUID        string
	QueueType    string // RANKED_SOLO_5x5, RANKED_FLEX_SR, ...
	Tier         string
	Rank         string
	LeaguePoints int
	Wins         int
	Losses       int
	UpdatedAt    time.Time
}

type Match struct {
	MatchID      string
	GameCreation time.Time
	GameDuration time.Duration
	QueueID      int
	GameVersion  string

	// RawData - исходный JSON Match-V5, ложится в matches.raw_data
	// (DECISIONS.md, отклонение 1).
	RawData json.RawMessage
}

func (m Match) DurationSeconds() int {
	return int(m.GameDuration.Seconds())
}

type MatchParticipant struct {
	MatchID      string
	PUUID        string
	ChampionName string
	Kills        int
	Deaths       int
	Assists      int
	Win          bool
	CS           int
	GoldEarned   int
}

func (p MatchParticipant) KDA() float64 {
	if p.Deaths == 0 {
		return float64(p.Kills + p.Assists)
	}

	return float64(p.Kills+p.Assists) / float64(p.Deaths)
}
