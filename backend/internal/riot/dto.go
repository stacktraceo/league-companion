package riot

import "encoding/json"

// DTO повторяют ответы Riot API один-в-один. Разбор в доменные модели —
// в пакете domain, чтобы форма чужого API не протекала дальше клиента.
//
// Для MVP парсим только поля из SPEC.md 7 (KDA, CS, чемпион, золото); всё
// остальное остаётся доступным через raw JSON матча (CLAUDE.md, отклонение 1).

// AccountDTO — ответ Account-V1 /riot/account/v1/accounts/by-riot-id/{name}/{tag}.
type AccountDTO struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// SummonerDTO — ответ Summoner-V4 /lol/summoner/v4/summoners/by-puuid/{puuid}.
type SummonerDTO struct {
	PUUID         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	SummonerLevel int64  `json:"summonerLevel"`
	RevisionDate  int64  `json:"revisionDate"`
}

// LeagueEntryDTO — элемент ответа League-V4 /lol/league/v4/entries/by-puuid/{puuid}.
type LeagueEntryDTO struct {
	PUUID        string `json:"puuid"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
}

// MatchDTO — ответ Match-V5 /lol/match/v5/matches/{matchId}.
type MatchDTO struct {
	Metadata MatchMetadataDTO `json:"metadata"`
	Info     MatchInfoDTO     `json:"info"`
}

// MatchMetadataDTO — блок metadata матча.
type MatchMetadataDTO struct {
	MatchID      string   `json:"matchId"`
	DataVersion  string   `json:"dataVersion"`
	Participants []string `json:"participants"`
}

// MatchInfoDTO — блок info матча.
type MatchInfoDTO struct {
	GameCreation int64 `json:"gameCreation"` // epoch millis

	// GameDuration — секунды для матчей с патча 11.20 и позже, миллисекунды для
	// более старых. Признак — наличие GameEndTimestamp; нормализуется в domain.
	GameDuration int64 `json:"gameDuration"`

	// GameEndTimestamp — epoch millis; отсутствует у матчей до патча 11.20.
	GameEndTimestamp int64 `json:"gameEndTimestamp"`

	GameVersion  string           `json:"gameVersion"`
	GameMode     string           `json:"gameMode"`
	QueueID      int              `json:"queueId"`
	PlatformID   string           `json:"platformId"`
	Participants []ParticipantDTO `json:"participants"`
}

// ParticipantDTO — статистика одного участника матча.
type ParticipantDTO struct {
	PUUID        string `json:"puuid"`
	ChampionName string `json:"championName"`
	ChampionID   int    `json:"championId"`
	TeamID       int    `json:"teamId"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
	Win          bool   `json:"win"`
	GoldEarned   int    `json:"goldEarned"`

	// CS складывается из миньонов и лесных монстров.
	TotalMinionsKilled   int `json:"totalMinionsKilled"`
	NeutralMinionsKilled int `json:"neutralMinionsKilled"`

	RiotIDGameName string `json:"riotIdGameName"`
	RiotIDTagline  string `json:"riotIdTagline"`
}

// CS — суммарное количество добитых миньонов и лесных монстров.
func (p ParticipantDTO) CS() int {
	return p.TotalMinionsKilled + p.NeutralMinionsKilled
}

// MatchDetail — распарсенный матч вместе с исходным JSON.
//
// Raw кладётся в matches.raw_data (CLAUDE.md, отклонение 1): расширение набора
// полей в будущем не должно требовать повторных запросов к Riot.
type MatchDetail struct {
	Match MatchDTO
	Raw   json.RawMessage
}
