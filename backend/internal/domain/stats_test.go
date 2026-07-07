package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// participation собирает участие для тестов агрегации: важны только чемпион,
// K/D/A и результат.
func participation(champion string, kills, deaths, assists int, win bool) MatchParticipant {
	return MatchParticipant{
		ChampionName: champion,
		Kills:        kills,
		Deaths:       deaths,
		Assists:      assists,
		Win:          win,
	}
}

func TestAggregateStatsEmptyInput(t *testing.T) {
	stats := AggregateStats(nil, 5)

	assert.Zero(t, stats.Games)
	assert.Zero(t, stats.Wins)
	assert.Zero(t, stats.Losses)
	assert.Zero(t, stats.WinRate)
	assert.Zero(t, stats.KDA)

	// Пустой срез, а не nil: клиенту проще перебирать пустой список.
	assert.NotNil(t, stats.TopChampions)
	assert.Empty(t, stats.TopChampions)
}

func TestAggregateStatsCountsGamesAndWinRate(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{
		participation("Ahri", 10, 2, 5, true),
		participation("Ahri", 6, 4, 2, false),
		participation("Zed", 8, 1, 3, true),
		participation("Zed", 4, 6, 1, true),
	}, 5)

	assert.Equal(t, 4, stats.Games)
	assert.Equal(t, 3, stats.Wins)
	assert.Equal(t, 1, stats.Losses)
	assert.InDelta(t, 0.75, stats.WinRate, 0.0001, "винрейт — доля 0..1, не проценты")

	assert.Equal(t, 28, stats.Kills)
	assert.Equal(t, 13, stats.Deaths)
	assert.Equal(t, 11, stats.Assists)
}

// KDA агрегатный: (ΣK+ΣA)/ΣD, как на op.gg. Среднее по матчам дало бы другое число —
// один матч без смертей задирал бы его вверх.
func TestAggregateStatsUsesAggregateKDA(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{
		participation("Ahri", 10, 2, 5, true),
		participation("Ahri", 8, 0, 4, true),
		participation("Ahri", 6, 4, 2, false),
	}, 5)

	// (10+8+6 + 5+4+2) / (2+0+4) = 35/6 = 5.83
	assert.InDelta(t, 35.0/6.0, stats.KDA, 0.0001)

	// Среднее по матчам дало бы 7.17: (7.5 + 12.0 + 2.0)/3, где 12.0 — матч
	// без смертей. Порог между двумя определениями и ловит подмену.
	assert.Less(t, stats.KDA, 6.0, "считаем агрегатный KDA, а не среднее по матчам")
}

// Ни одной смерти за период — делить не на что; отдаём сумму убийств и ассистов,
// как это уже делает MatchParticipant.KDA().
func TestAggregateStatsPerfectKDA(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{
		participation("Ahri", 10, 0, 5, true),
		participation("Ahri", 3, 0, 2, true),
	}, 5)

	assert.Zero(t, stats.Deaths)
	assert.InDelta(t, 20.0, stats.KDA, 0.0001)
}

func TestAggregateStatsTopChampionsOrderedByGames(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{
		participation("Zed", 1, 1, 1, true),
		participation("Ahri", 2, 1, 2, true),
		participation("Ahri", 2, 1, 2, false),
		participation("Ahri", 2, 1, 2, true),
		participation("Yone", 3, 1, 3, false),
		participation("Yone", 3, 1, 3, true),
	}, 5)

	require.Len(t, stats.TopChampions, 3)

	assert.Equal(t, "Ahri", stats.TopChampions[0].ChampionName)
	assert.Equal(t, 3, stats.TopChampions[0].Games)
	assert.Equal(t, 2, stats.TopChampions[0].Wins)
	assert.InDelta(t, 2.0/3.0, stats.TopChampions[0].WinRate, 0.0001)
	// (2+2+2 + 2+2+2) / (1+1+1) = 4
	assert.InDelta(t, 4.0, stats.TopChampions[0].KDA, 0.0001)

	assert.Equal(t, "Yone", stats.TopChampions[1].ChampionName)
	assert.Equal(t, "Zed", stats.TopChampions[2].ChampionName)
}

// При равном числе игр порядок обязан быть определённым, иначе тесты мигают,
// а клиент видит разный топ на одинаковых данных.
func TestAggregateStatsTopChampionsTieBreak(t *testing.T) {
	items := []MatchParticipant{
		// Одна игра у каждого, KDA убывает: Yasuo 10 > Ahri 5 > Braum 1.
		participation("Braum", 1, 1, 0, true),
		participation("Ahri", 4, 1, 1, true),
		participation("Yasuo", 8, 1, 2, true),
	}

	first := AggregateStats(items, 5)
	require.Len(t, first.TopChampions, 3)
	assert.Equal(t, []string{"Yasuo", "Ahri", "Braum"}, championNames(first.TopChampions),
		"при равном числе игр — по KDA вниз")

	// Тот же вход в другом порядке обязан дать тот же результат.
	shuffled := AggregateStats([]MatchParticipant{items[2], items[0], items[1]}, 5)
	assert.Equal(t, championNames(first.TopChampions), championNames(shuffled.TopChampions))
}

// Полное совпадение и по играм, и по KDA разрешается именем — иначе порядок
// зависел бы от обхода map'ы.
func TestAggregateStatsTopChampionsTieBreakByName(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{
		participation("Yone", 2, 1, 1, true),
		participation("Ahri", 2, 1, 1, true),
		participation("Kled", 2, 1, 1, true),
	}, 5)

	assert.Equal(t, []string{"Ahri", "Kled", "Yone"}, championNames(stats.TopChampions))
}

func TestAggregateStatsLimitsTopChampions(t *testing.T) {
	var items []MatchParticipant

	// Шесть чемпионов, у каждого своё число игр — топ-5 обязан отрезать последнего.
	for i, champion := range []string{"C6", "C5", "C4", "C3", "C2", "C1"} {
		for range i + 1 {
			items = append(items, participation(champion, 1, 1, 1, true))
		}
	}

	stats := AggregateStats(items, 5)

	require.Len(t, stats.TopChampions, 5)
	assert.Equal(t, []string{"C1", "C2", "C3", "C4", "C5"}, championNames(stats.TopChampions))
}

// topN больше числа чемпионов — не ошибка, просто отдаём всех.
func TestAggregateStatsFewerChampionsThanLimit(t *testing.T) {
	stats := AggregateStats([]MatchParticipant{participation("Ahri", 1, 1, 1, true)}, 5)

	assert.Len(t, stats.TopChampions, 1)
}

// Бессмысленный topN не должен приводить к панике или отрицательной длине.
func TestAggregateStatsNonPositiveLimit(t *testing.T) {
	items := []MatchParticipant{participation("Ahri", 1, 1, 1, true)}

	for _, topN := range []int{0, -1} {
		stats := AggregateStats(items, topN)

		assert.Equal(t, 1, stats.Games, "сама агрегация считается независимо от topN")
		assert.Empty(t, stats.TopChampions)
	}
}

func championNames(champions []ChampionStats) []string {
	names := make([]string, 0, len(champions))
	for _, champion := range champions {
		names = append(names, champion.ChampionName)
	}

	return names
}
