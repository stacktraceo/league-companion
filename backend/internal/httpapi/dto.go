package httpapi

import (
	"time"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
)

// JSON-теги живут здесь, а не на доменных структурах: формат API не должен
// диктоваться формой таблиц, и наоборот.

// createSummonerRequest — тело POST /api/v1/summoners (SPEC.md 3.4).
type createSummonerRequest struct {
	// RiotID — игровое имя без тега (GameName из GameName#TagLine).
	RiotID  string `json:"riotId"`
	TagLine string `json:"tagLine"`
	Region  string `json:"region"`
}

// SummonerResponse — профиль саммонера вместе с ранговым снапшотом.
type SummonerResponse struct {
	PUUID         string           `json:"puuid"`
	RiotID        string           `json:"riotId"`
	TagLine       string           `json:"tagLine"`
	Region        string           `json:"region"`
	SummonerLevel int              `json:"summonerLevel"`
	ProfileIconID int              `json:"profileIconId"`
	LastSyncedAt  *time.Time       `json:"lastSyncedAt"`
	CreatedAt     time.Time        `json:"createdAt"`
	Ranked        []RankedResponse `json:"ranked"`

	// Stale — Riot был недоступен, отдан сохранённый снапшот (SPEC.md 3.4).
	Stale bool `json:"stale,omitempty"`
}

// RankedResponse — ранг в одной очереди.
type RankedResponse struct {
	QueueType    string    `json:"queueType"`
	Tier         string    `json:"tier"`
	Rank         string    `json:"rank"`
	LeaguePoints int       `json:"leaguePoints"`
	Wins         int       `json:"wins"`
	Losses       int       `json:"losses"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MatchListResponse — страница ленты матчей.
type MatchListResponse struct {
	Items  []MatchListItemResponse `json:"items"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
	Total  int                     `json:"total"`
}

// MatchListItemResponse — матч глазами одного саммонера.
type MatchListItemResponse struct {
	MatchID             string    `json:"matchId"`
	GameCreation        time.Time `json:"gameCreation"`
	GameDurationSeconds int       `json:"gameDurationSeconds"`
	QueueID             int       `json:"queueId"`
	GameVersion         string    `json:"gameVersion"`

	ChampionName string  `json:"championName"`
	Kills        int     `json:"kills"`
	Deaths       int     `json:"deaths"`
	Assists      int     `json:"assists"`
	KDA          float64 `json:"kda"`
	Win          bool    `json:"win"`
	CS           int     `json:"cs"`
	GoldEarned   int     `json:"goldEarned"`
}

func summonerResponse(summoner domain.Summoner, ranked []domain.RankedStat, stale bool) SummonerResponse {
	response := SummonerResponse{
		PUUID:         summoner.PUUID,
		RiotID:        summoner.RiotID,
		TagLine:       summoner.TagLine,
		Region:        summoner.Region,
		SummonerLevel: summoner.SummonerLevel,
		ProfileIconID: summoner.ProfileIconID,
		LastSyncedAt:  summoner.LastSyncedAt,
		CreatedAt:     summoner.CreatedAt,
		// Пустой срез, а не null: клиенту без ранга проще перебирать пустой список.
		Ranked: make([]RankedResponse, 0, len(ranked)),
		Stale:  stale,
	}

	for _, stat := range ranked {
		response.Ranked = append(response.Ranked, RankedResponse{
			QueueType:    stat.QueueType,
			Tier:         stat.Tier,
			Rank:         stat.Rank,
			LeaguePoints: stat.LeaguePoints,
			Wins:         stat.Wins,
			Losses:       stat.Losses,
			UpdatedAt:    stat.UpdatedAt,
		})
	}

	return response
}

func matchListResponse(items []storage.MatchListItem, limit, offset, total int) MatchListResponse {
	response := MatchListResponse{
		Items:  make([]MatchListItemResponse, 0, len(items)),
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}

	for _, item := range items {
		participant := domain.MatchParticipant{
			Kills:   item.Kills,
			Deaths:  item.Deaths,
			Assists: item.Assists,
		}

		response.Items = append(response.Items, MatchListItemResponse{
			MatchID:             item.MatchID,
			GameCreation:        item.GameCreation,
			GameDurationSeconds: int(item.GameDuration.Seconds()),
			QueueID:             item.QueueID,
			GameVersion:         item.GameVersion,
			ChampionName:        item.ChampionName,
			Kills:               item.Kills,
			Deaths:              item.Deaths,
			Assists:             item.Assists,
			KDA:                 participant.KDA(),
			Win:                 item.Win,
			CS:                  item.CS,
			GoldEarned:          item.GoldEarned,
		})
	}

	return response
}
