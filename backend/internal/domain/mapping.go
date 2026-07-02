package domain

import (
	"errors"
	"time"

	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

// ErrEmptyMatch возвращается, если Riot прислал матч без идентификатора.
var ErrEmptyMatch = errors.New("domain: матч без matchId")

// SummonerFromRiot собирает саммонера из ответов Account-V1 и Summoner-V4.
//
// Регион приходит от вызывающего: Riot его в этих ответах не возвращает.
func SummonerFromRiot(account riot.AccountDTO, summoner riot.SummonerDTO, region string) Summoner {
	return Summoner{
		PUUID:         account.PUUID,
		RiotID:        account.GameName,
		TagLine:       account.TagLine,
		Region:        region,
		SummonerLevel: int(summoner.SummonerLevel),
		ProfileIconID: summoner.ProfileIconID,
	}
}

// RankedStatsFromRiot переводит записи League-V4 в ранговые снапшоты.
// updatedAt передаётся снаружи, чтобы маппинг оставался детерминированным.
func RankedStatsFromRiot(puuid string, entries []riot.LeagueEntryDTO, updatedAt time.Time) []RankedStat {
	if len(entries) == 0 {
		return nil
	}

	stats := make([]RankedStat, 0, len(entries))
	for _, entry := range entries {
		stats = append(stats, RankedStat{
			PUUID:        puuid,
			QueueType:    entry.QueueType,
			Tier:         entry.Tier,
			Rank:         entry.Rank,
			LeaguePoints: entry.LeaguePoints,
			Wins:         entry.Wins,
			Losses:       entry.Losses,
			UpdatedAt:    updatedAt,
		})
	}

	return stats
}

// MatchFromRiot переводит ответ Match-V5 в доменный матч вместе с сырым JSON.
func MatchFromRiot(detail riot.MatchDetail) (Match, error) {
	matchID := detail.Match.Metadata.MatchID
	if matchID == "" {
		return Match{}, ErrEmptyMatch
	}

	info := detail.Match.Info

	return Match{
		MatchID:      matchID,
		GameCreation: time.UnixMilli(info.GameCreation).UTC(),
		GameDuration: gameDuration(info),
		QueueID:      info.QueueID,
		GameVersion:  info.GameVersion,
		RawData:      detail.Raw,
	}, nil
}

// MatchParticipantsFromRiot возвращает всех участников матча.
//
// В match_participants попадут только отслеживаемые саммонеры (на таблице стоит
// FK на summoners), фильтрация — забота вызывающего; полный состав обеих команд
// отдаётся из raw_data.
func MatchParticipantsFromRiot(detail riot.MatchDetail) ([]MatchParticipant, error) {
	matchID := detail.Match.Metadata.MatchID
	if matchID == "" {
		return nil, ErrEmptyMatch
	}

	participants := make([]MatchParticipant, 0, len(detail.Match.Info.Participants))
	for _, p := range detail.Match.Info.Participants {
		participants = append(participants, MatchParticipant{
			MatchID:      matchID,
			PUUID:        p.PUUID,
			ChampionName: p.ChampionName,
			Kills:        p.Kills,
			Deaths:       p.Deaths,
			Assists:      p.Assists,
			Win:          p.Win,
			CS:           p.CS(),
			GoldEarned:   p.GoldEarned,
		})
	}

	return participants, nil
}

// gameDuration нормализует известную особенность Match-V5: у матчей с патча 11.20
// и позже gameDuration в секундах, у более старых — в миллисекундах. Отличить их
// можно по gameEndTimestamp, которого у старых матчей нет.
func gameDuration(info riot.MatchInfoDTO) time.Duration {
	if info.GameEndTimestamp == 0 {
		return time.Duration(info.GameDuration) * time.Millisecond
	}

	return time.Duration(info.GameDuration) * time.Second
}
