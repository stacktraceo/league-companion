package domain

import (
	"cmp"
	"slices"
)

type Stats struct {
	Games  int
	Wins   int
	Losses int

	// WinRate - доля побед, 0..1. Проценты форматирует клиент: две формы одного
	// числа в API рано или поздно расходятся.
	WinRate float64

	// Суммы за период - из них считается KDA, и по ним же клиент может показать
	// «в среднем за игру», не запрашивая матчи.
	Kills   int
	Deaths  int
	Assists int

	// KDA - агрегатный (ΣK+ΣA)/ΣD за период, как считают op.gg и u.gg.
	KDA float64

	TopChampions []ChampionStats
}

type ChampionStats struct {
	ChampionName string
	Games        int
	Wins         int
	WinRate      float64
	KDA          float64
}

func AggregateStats(participations []MatchParticipant, topN int) Stats {
	stats := Stats{
		Games:        len(participations),
		TopChampions: []ChampionStats{},
	}

	// Накопитель по чемпиону: считаем суммы, KDA и винрейт выводим в конце -
	// усреднять уже усреднённое нельзя.
	type totals struct {
		games   int
		wins    int
		kills   int
		deaths  int
		assists int
	}

	byChampion := make(map[string]*totals)

	for _, p := range participations {
		stats.Kills += p.Kills
		stats.Deaths += p.Deaths
		stats.Assists += p.Assists

		if p.Win {
			stats.Wins++
		}

		champion, ok := byChampion[p.ChampionName]
		if !ok {
			champion = &totals{}
			byChampion[p.ChampionName] = champion
		}

		champion.games++
		champion.kills += p.Kills
		champion.deaths += p.Deaths
		champion.assists += p.Assists

		if p.Win {
			champion.wins++
		}
	}

	stats.Losses = stats.Games - stats.Wins
	stats.WinRate = ratio(stats.Wins, stats.Games)
	stats.KDA = aggregateKDA(stats.Kills, stats.Deaths, stats.Assists)

	if topN <= 0 {
		return stats
	}

	champions := make([]ChampionStats, 0, len(byChampion))

	for name, t := range byChampion {
		champions = append(champions, ChampionStats{
			ChampionName: name,
			Games:        t.games,
			Wins:         t.wins,
			WinRate:      ratio(t.wins, t.games),
			KDA:          aggregateKDA(t.kills, t.deaths, t.assists),
		})
	}

	slices.SortFunc(champions, func(a, b ChampionStats) int {
		if byGames := cmp.Compare(b.Games, a.Games); byGames != 0 {
			return byGames
		}

		if byKDA := cmp.Compare(b.KDA, a.KDA); byKDA != 0 {
			return byKDA
		}

		return cmp.Compare(a.ChampionName, b.ChampionName)
	})

	stats.TopChampions = champions[:min(topN, len(champions))]

	return stats
}

func aggregateKDA(kills, deaths, assists int) float64 {
	if deaths == 0 {
		return float64(kills + assists)
	}

	return float64(kills+assists) / float64(deaths)
}

func ratio(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total)
}
