// Package domain описывает модели предметной области — то, что лежит в БД и
// уезжает в API. Форма ответов Riot дальше пакета riot не протекает.
package domain

import (
	"encoding/json"
	"time"
)

// Summoner — отслеживаемый саммонер.
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

// FullRiotID возвращает Riot ID в привычном виде GameName#TagLine.
func (s Summoner) FullRiotID() string {
	return s.RiotID + "#" + s.TagLine
}

// RankedStat — ранговый снапшот по одной очереди.
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

// Match — матч целиком, общий для всех участников.
type Match struct {
	MatchID      string
	GameCreation time.Time
	GameDuration time.Duration
	QueueID      int
	GameVersion  string

	// RawData — исходный JSON Match-V5, ложится в matches.raw_data
	// (CLAUDE.md, отклонение 1).
	RawData json.RawMessage
}

// DurationSeconds возвращает длительность в секундах — в таком виде она лежит
// в колонке matches.game_duration.
func (m Match) DurationSeconds() int {
	return int(m.GameDuration.Seconds())
}

// MatchParticipant — участие одного игрока в матче.
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

// KDA — классическое (K+A)/D. При нуле смертей делить не на что, поэтому
// возвращаем сумму убийств и ассистов (perfect KDA).
func (p MatchParticipant) KDA() float64 {
	if p.Deaths == 0 {
		return float64(p.Kills + p.Assists)
	}

	return float64(p.Kills+p.Assists) / float64(p.Deaths)
}
