package storage

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
)

// testPool поднимает пул на тестовой базе и оставляет схему пустой.
//
// Без TEST_DATABASE_URL тесты пропускаются: `go test ./...` должен оставаться
// зелёным без поднятого Postgres. Поднять базу для прогона:
//
//	docker run --rm -d --name lc-test-pg -p 55432:5432 \
//	  -e POSTGRES_USER=league -e POSTGRES_PASSWORD=league -e POSTGRES_DB=league_test postgres:16
//	TEST_DATABASE_URL='postgres://league:league@localhost:55432/league_test?sslmode=disable' \
//	  go test ./internal/storage/...
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан — интеграционные тесты пропущены")
	}

	require.NoError(t, Migrate(dsn, slog.New(slog.NewTextHandler(io.Discard, nil))))

	pool, err := Connect(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	_, err = pool.Exec(context.Background(),
		`TRUNCATE summoners, ranked_stats, matches, match_participants CASCADE`)
	require.NoError(t, err)

	return pool
}

func testSummoner(puuid string) domain.Summoner {
	return domain.Summoner{
		PUUID:         puuid,
		RiotID:        "Test Summoner",
		TagLine:       "EUW",
		Region:        "euw1",
		SummonerLevel: 100,
		ProfileIconID: 42,
	}
}

func testMatch(matchID string, creation time.Time) domain.Match {
	return domain.Match{
		MatchID:      matchID,
		GameCreation: creation,
		GameDuration: 25 * time.Minute,
		QueueID:      420,
		GameVersion:  "14.1.556.1234",
		RawData:      json.RawMessage(`{"metadata":{"matchId":"` + matchID + `"}}`),
	}
}

func testParticipant(matchID, puuid string) domain.MatchParticipant {
	return domain.MatchParticipant{
		MatchID:      matchID,
		PUUID:        puuid,
		ChampionName: "Ahri",
		Kills:        11,
		Deaths:       3,
		Assists:      9,
		Win:          true,
		CS:           201,
		GoldEarned:   14320,
	}
}

// mustUpsert сохраняет саммонера и возвращает признак «создан впервые».
func mustUpsert(t *testing.T, ctx context.Context, repo *Summoners, summoner domain.Summoner) bool {
	t.Helper()

	stored, created, err := repo.Upsert(ctx, summoner)
	require.NoError(t, err)

	// Upsert обязан вернуть строку, а не то, что ему передали: created_at
	// заполняет база, и именно он уходит в ответ API на POST /summoners.
	assert.Equal(t, summoner.PUUID, stored.PUUID)
	assert.False(t, stored.CreatedAt.IsZero(), "created_at приходит из базы")

	return created
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()

	var total int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&total))

	return total
}

func TestSummonerUpsertRefreshesProfile(t *testing.T) {
	pool := testPool(t)
	repo := NewSummoners(pool)
	ctx := context.Background()

	summoner := testSummoner("puuid-1")
	assert.True(t, mustUpsert(t, ctx, repo, summoner), "первое добавление — создание")

	stored, err := repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	assert.Equal(t, 100, stored.SummonerLevel)
	assert.Nil(t, stored.LastSyncedAt, "до первой синхронизации отметки нет")
	createdAt := stored.CreatedAt

	// Повторное добавление обязано подтянуть свежий профиль, а не промолчать.
	summoner.SummonerLevel = 137
	summoner.ProfileIconID = 7
	assert.False(t, mustUpsert(t, ctx, repo, summoner),
		"повторное — обновление, по нему хендлер отличает 200 от 201")

	updated, err := repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	assert.Equal(t, 137, updated.SummonerLevel)
	assert.Equal(t, 7, updated.ProfileIconID)
	assert.Equal(t, createdAt, updated.CreatedAt, "created_at принадлежит моменту создания")
	assert.Equal(t, 1, countRows(t, pool, "summoners"))
}

// ByRiotID — единственный способ найти снапшот, когда Riot лежит и резолвить puuid
// нечем (SPEC.md 3.4).
func TestSummonerByRiotIDIgnoresCase(t *testing.T) {
	repo := NewSummoners(testPool(t))
	ctx := context.Background()

	mustUpsert(t, ctx, repo, testSummoner("puuid-1"))

	// Пользователь набирает ник руками, а в базе лежит написание, которое вернул Riot.
	stored, err := repo.ByRiotID(ctx, "EUW1", "test summoner", "euw")
	require.NoError(t, err)
	assert.Equal(t, "puuid-1", stored.PUUID)

	_, err = repo.ByRiotID(ctx, "euw1", "Кто-то другой", "EUW")
	assert.ErrorIs(t, err, ErrNotFound)

	// Тег — часть идентификатора, а не украшение: тот же ник с другим тегом это
	// другой человек.
	_, err = repo.ByRiotID(ctx, "euw1", "Test Summoner", "RU")
	assert.ErrorIs(t, err, ErrNotFound)
}

// После переименования старая запись остаётся со своим puuid, и под один Riot ID
// могут подойти две строки. Берём синхронизированную позже.
func TestSummonerByRiotIDPrefersFreshestSnapshot(t *testing.T) {
	repo := NewSummoners(testPool(t))
	ctx := context.Background()

	mustUpsert(t, ctx, repo, testSummoner("puuid-старый"))
	mustUpsert(t, ctx, repo, testSummoner("puuid-новый"))

	// У «старого» отметка синхронизации есть, у «нового» пока нет — свежим считается
	// тот, о ком известно больше.
	require.NoError(t, repo.MarkSynced(ctx, "puuid-старый", time.Now()))

	stored, err := repo.ByRiotID(ctx, "euw1", "Test Summoner", "EUW")
	require.NoError(t, err)
	assert.Equal(t, "puuid-старый", stored.PUUID)

	// А когда синхронизирован и второй, побеждает он — NULLS LAST не должен
	// перевешивать саму дату.
	require.NoError(t, repo.MarkSynced(ctx, "puuid-новый", time.Now().Add(time.Minute)))

	stored, err = repo.ByRiotID(ctx, "euw1", "Test Summoner", "EUW")
	require.NoError(t, err)
	assert.Equal(t, "puuid-новый", stored.PUUID)
}

func TestSummonerNotFound(t *testing.T) {
	repo := NewSummoners(testPool(t))

	_, err := repo.ByPUUID(context.Background(), "нет-такого")
	assert.ErrorIs(t, err, ErrNotFound)

	assert.ErrorIs(t, repo.MarkSynced(context.Background(), "нет-такого", time.Now()), ErrNotFound)
}

func TestMarkSyncedAndTrackedPUUIDs(t *testing.T) {
	repo := NewSummoners(testPool(t))
	ctx := context.Background()

	mustUpsert(t, ctx, repo, testSummoner("puuid-1"))
	mustUpsert(t, ctx, repo, testSummoner("puuid-2"))

	at := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, repo.MarkSynced(ctx, "puuid-1", at))

	stored, err := repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	require.NotNil(t, stored.LastSyncedAt)
	assert.WithinDuration(t, at, *stored.LastSyncedAt, time.Millisecond)

	tracked, err := repo.TrackedPUUIDs(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"puuid-1": {}, "puuid-2": {}}, tracked)

	all, err := repo.All(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestRankedStatsReplaceDropsStaleQueues(t *testing.T) {
	pool := testPool(t)
	mustUpsert(t, context.Background(), NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewRankedStats(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	require.NoError(t, repo.Replace(ctx, "puuid-1", []domain.RankedStat{
		{PUUID: "puuid-1", QueueType: "RANKED_SOLO_5x5", Tier: "GOLD", Rank: "II",
			LeaguePoints: 47, Wins: 63, Losses: 58, UpdatedAt: now},
		{PUUID: "puuid-1", QueueType: "RANKED_FLEX_SR", Tier: "SILVER", Rank: "I",
			LeaguePoints: 12, Wins: 5, Losses: 4, UpdatedAt: now},
	}))

	stats, err := repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, "RANKED_FLEX_SR", stats[0].QueueType)
	assert.Equal(t, "GOLD", stats[1].Tier)
	assert.Equal(t, 47, stats[1].LeaguePoints)

	// Игрок перестал играть флекс — старая запись не должна пережить замену.
	require.NoError(t, repo.Replace(ctx, "puuid-1", []domain.RankedStat{
		{PUUID: "puuid-1", QueueType: "RANKED_SOLO_5x5", Tier: "PLATINUM", Rank: "IV",
			LeaguePoints: 3, Wins: 70, Losses: 60, UpdatedAt: now},
	}))

	stats, err = repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "PLATINUM", stats[0].Tier)

	// Саммонер без ранга — валидный случай.
	require.NoError(t, repo.Replace(ctx, "puuid-1", nil))

	stats, err = repo.ByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestMatchInsertIsIdempotent(t *testing.T) {
	pool := testPool(t)
	mustUpsert(t, context.Background(), NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewMatches(pool)
	ctx := context.Background()
	match := testMatch("EUW1_1", time.Now().UTC().Add(-time.Hour))
	participants := []domain.MatchParticipant{testParticipant("EUW1_1", "puuid-1")}

	require.NoError(t, repo.Insert(ctx, match, participants))
	require.NoError(t, repo.Insert(ctx, match, participants), "повторная вставка не должна падать")

	assert.Equal(t, 1, countRows(t, pool, "matches"))
	assert.Equal(t, 1, countRows(t, pool, "match_participants"))
}

// Двое отслеживаемых саммонеров могут встретиться в одном матче и синхронизироваться
// параллельно (DECISIONS.md, отклонение 2): матч не дублируется, участие добавляется.
func TestMatchInsertAddsSecondTrackedParticipant(t *testing.T) {
	pool := testPool(t)
	summoners := NewSummoners(pool)
	ctx := context.Background()

	mustUpsert(t, ctx, summoners, testSummoner("puuid-1"))
	mustUpsert(t, ctx, summoners, testSummoner("puuid-2"))

	repo := NewMatches(pool)
	match := testMatch("EUW1_1", time.Now().UTC())

	require.NoError(t, repo.Insert(ctx, match, []domain.MatchParticipant{testParticipant("EUW1_1", "puuid-1")}))
	require.NoError(t, repo.Insert(ctx, match, []domain.MatchParticipant{testParticipant("EUW1_1", "puuid-2")}))

	assert.Equal(t, 1, countRows(t, pool, "matches"))
	assert.Equal(t, 2, countRows(t, pool, "match_participants"))
}

func TestKnownIDs(t *testing.T) {
	pool := testPool(t)
	mustUpsert(t, context.Background(), NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewMatches(pool)
	ctx := context.Background()

	require.NoError(t, repo.Insert(ctx, testMatch("EUW1_1", time.Now().UTC()), nil))

	known, err := repo.KnownIDs(ctx, []string{"EUW1_1", "EUW1_2"})
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"EUW1_1": {}}, known)

	empty, err := repo.KnownIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestListByPUUIDPaginatesNewestFirst(t *testing.T) {
	pool := testPool(t)
	mustUpsert(t, context.Background(), NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewMatches(pool)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second)

	for i, id := range []string{"EUW1_1", "EUW1_2", "EUW1_3"} {
		match := testMatch(id, base.Add(time.Duration(i)*time.Hour))
		require.NoError(t, repo.Insert(ctx, match, []domain.MatchParticipant{testParticipant(id, "puuid-1")}))
	}

	total, err := repo.CountByPUUID(ctx, "puuid-1")
	require.NoError(t, err)
	assert.Equal(t, 3, total)

	first, err := repo.ListByPUUID(ctx, "puuid-1", 2, 0)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, "EUW1_3", first[0].MatchID, "свежие первыми")
	assert.Equal(t, "EUW1_2", first[1].MatchID)
	assert.Equal(t, 25*time.Minute, first[0].GameDuration)
	assert.Equal(t, "Ahri", first[0].ChampionName)

	second, err := repo.ListByPUUID(ctx, "puuid-1", 2, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Equal(t, "EUW1_1", second[0].MatchID)

	beyond, err := repo.ListByPUUID(ctx, "puuid-1", 2, 10)
	require.NoError(t, err)
	assert.Empty(t, beyond)
}

func TestRawByID(t *testing.T) {
	pool := testPool(t)
	mustUpsert(t, context.Background(), NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewMatches(pool)
	ctx := context.Background()
	match := testMatch("EUW1_1", time.Now().UTC())
	require.NoError(t, repo.Insert(ctx, match, nil))

	raw, err := repo.RawByID(ctx, "EUW1_1")
	require.NoError(t, err)
	assert.JSONEq(t, string(match.RawData), string(raw))

	_, err = repo.RawByID(ctx, "нет-такого")
	assert.ErrorIs(t, err, ErrNotFound)
}

// Матч общий для всех участников и переживает удаление саммонера; его участие — нет.
func TestDeletingSummonerCascadesToParticipation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mustUpsert(t, ctx, NewSummoners(pool), testSummoner("puuid-1"))

	repo := NewMatches(pool)
	require.NoError(t, repo.Insert(ctx, testMatch("EUW1_1", time.Now().UTC()),
		[]domain.MatchParticipant{testParticipant("EUW1_1", "puuid-1")}))

	_, err := pool.Exec(ctx, `DELETE FROM summoners WHERE puuid = $1`, "puuid-1")
	require.NoError(t, err)

	assert.Equal(t, 0, countRows(t, pool, "match_participants"))
	assert.Equal(t, 1, countRows(t, pool, "matches"))
}

// ParticipationsSince отбирает по времени матча, а не по времени вставки: агрегация
// за 30 дней не должна захватывать матч годичной давности, догруженный сегодня.
func TestParticipationsSinceFiltersByGameCreation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mustUpsert(t, ctx, NewSummoners(pool), testSummoner("puuid-1"))
	mustUpsert(t, ctx, NewSummoners(pool), testSummoner("puuid-2"))

	repo := NewMatches(pool)
	now := time.Now().UTC()

	// Внутри окна, на границе и снаружи.
	for id, creation := range map[string]time.Time{
		"EUW1_fresh":  now.Add(-time.Hour),
		"EUW1_edge":   now.AddDate(0, 0, -30).Add(time.Minute),
		"EUW1_stale":  now.AddDate(0, 0, -31),
		"EUW1_others": now.Add(-2 * time.Hour),
	} {
		puuid := "puuid-1"
		if id == "EUW1_others" {
			puuid = "puuid-2"
		}

		require.NoError(t, repo.Insert(ctx, testMatch(id, creation),
			[]domain.MatchParticipant{testParticipant(id, puuid)}))
	}

	since := now.AddDate(0, 0, -30)

	participations, err := repo.ParticipationsSince(ctx, "puuid-1", since)
	require.NoError(t, err)

	ids := make([]string, 0, len(participations))
	for _, p := range participations {
		ids = append(ids, p.MatchID)
	}

	// Свежие первыми, чужое участие не попало, матч за границей окна отброшен.
	assert.Equal(t, []string{"EUW1_fresh", "EUW1_edge"}, ids)

	require.NotEmpty(t, participations)
	assert.Equal(t, 11, participations[0].Kills, "поля участия доезжают целиком")
	assert.Equal(t, "Ahri", participations[0].ChampionName)
}

// Саммонер без матчей за период — пустой результат, а не ошибка.
func TestParticipationsSinceWithoutMatches(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mustUpsert(t, ctx, NewSummoners(pool), testSummoner("puuid-1"))

	participations, err := NewMatches(pool).ParticipationsSince(
		ctx, "puuid-1", time.Now().UTC().AddDate(0, 0, -30))
	require.NoError(t, err)
	assert.Empty(t, participations)
}
