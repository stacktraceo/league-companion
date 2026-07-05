package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

const testRegion = "euw1"

var testNow = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

// fakeRiot — управляемый клиент Riot: отдаёт заготовленные ответы и считает вызовы.
type fakeRiot struct {
	mu sync.Mutex

	account  riot.AccountDTO
	summoner riot.SummonerDTO
	entries  []riot.LeagueEntryDTO
	matchIDs []string
	matches  map[string]riot.MatchDetail

	accountErr error
	profileErr error
	leagueErr  error
	idsErr     error
	matchErrs  map[string]error

	requestedMatches []string
}

func (f *fakeRiot) GetAccountByRiotID(context.Context, string, string, string) (*riot.AccountDTO, error) {
	if f.accountErr != nil {
		return nil, f.accountErr
	}

	return &f.account, nil
}

func (f *fakeRiot) GetSummonerByPUUID(context.Context, string, string) (*riot.SummonerDTO, error) {
	if f.profileErr != nil {
		return nil, f.profileErr
	}

	return &f.summoner, nil
}

func (f *fakeRiot) GetLeagueEntriesByPUUID(context.Context, string, string) ([]riot.LeagueEntryDTO, error) {
	if f.leagueErr != nil {
		return nil, f.leagueErr
	}

	return f.entries, nil
}

func (f *fakeRiot) GetMatchIDsByPUUID(context.Context, string, string, int, int) ([]string, error) {
	if f.idsErr != nil {
		return nil, f.idsErr
	}

	return f.matchIDs, nil
}

func (f *fakeRiot) GetMatch(_ context.Context, _, matchID string) (*riot.MatchDetail, error) {
	f.mu.Lock()
	f.requestedMatches = append(f.requestedMatches, matchID)
	f.mu.Unlock()

	if err, ok := f.matchErrs[matchID]; ok {
		return nil, err
	}

	detail, ok := f.matches[matchID]
	if !ok {
		return nil, riot.ErrNotFound
	}

	return &detail, nil
}

func (f *fakeRiot) Requested() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.requestedMatches...)
}

// fakeRepos заменяет все три репозитория разом — в тестах они всегда нужны вместе.
type fakeRepos struct {
	mu sync.Mutex

	summoners map[string]domain.Summoner
	ranked    map[string][]domain.RankedStat
	matches   map[string]domain.Match
	inserted  map[string][]domain.MatchParticipant
	syncedAt  map[string]time.Time

	upsertErr error
	insertErr error
}

func newFakeRepos() *fakeRepos {
	return &fakeRepos{
		summoners: make(map[string]domain.Summoner),
		ranked:    make(map[string][]domain.RankedStat),
		matches:   make(map[string]domain.Match),
		inserted:  make(map[string][]domain.MatchParticipant),
		syncedAt:  make(map[string]time.Time),
	}
}

func (f *fakeRepos) Upsert(_ context.Context, summoner domain.Summoner) (bool, error) {
	if f.upsertErr != nil {
		return false, f.upsertErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	_, existed := f.summoners[summoner.PUUID]
	f.summoners[summoner.PUUID] = summoner

	return !existed, nil
}

func (f *fakeRepos) ByPUUID(_ context.Context, puuid string) (domain.Summoner, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	summoner, ok := f.summoners[puuid]
	if !ok {
		return domain.Summoner{}, errors.New("нет такого саммонера")
	}

	return summoner, nil
}

func (f *fakeRepos) TrackedPUUIDs(context.Context) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	tracked := make(map[string]struct{}, len(f.summoners))
	for puuid := range f.summoners {
		tracked[puuid] = struct{}{}
	}

	return tracked, nil
}

func (f *fakeRepos) MarkSynced(_ context.Context, puuid string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.syncedAt[puuid] = at

	return nil
}

func (f *fakeRepos) Replace(_ context.Context, puuid string, stats []domain.RankedStat) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ranked[puuid] = stats

	return nil
}

func (f *fakeRepos) KnownIDs(_ context.Context, ids []string) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	known := make(map[string]struct{})

	for _, id := range ids {
		if _, ok := f.matches[id]; ok {
			known[id] = struct{}{}
		}
	}

	return known, nil
}

func (f *fakeRepos) Insert(
	_ context.Context,
	match domain.Match,
	participants []domain.MatchParticipant,
) error {
	if f.insertErr != nil {
		return f.insertErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.matches[match.MatchID] = match
	f.inserted[match.MatchID] = participants

	return nil
}

func newTestService(client *fakeRiot, repos *fakeRepos) *Service {
	service := NewService(client, repos, repos, repos,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.now = func() time.Time { return testNow }

	return service
}

// matchDetail собирает ответ Match-V5 с заданными участниками.
func matchDetail(matchID string, puuids ...string) riot.MatchDetail {
	participants := make([]riot.ParticipantDTO, 0, len(puuids))
	for _, puuid := range puuids {
		participants = append(participants, riot.ParticipantDTO{
			PUUID:              puuid,
			ChampionName:       "Ahri",
			Kills:              11,
			Deaths:             3,
			Assists:            9,
			Win:                true,
			GoldEarned:         14320,
			TotalMinionsKilled: 180,
		})
	}

	return riot.MatchDetail{
		Match: riot.MatchDTO{
			Metadata: riot.MatchMetadataDTO{MatchID: matchID, Participants: puuids},
			Info: riot.MatchInfoDTO{
				GameCreation:     testNow.UnixMilli(),
				GameDuration:     1500,
				GameEndTimestamp: testNow.UnixMilli(),
				GameVersion:      "14.1.556.1234",
				QueueID:          420,
				Participants:     participants,
			},
		},
		Raw: json.RawMessage(`{"metadata":{"matchId":"` + matchID + `"}}`),
	}
}

func TestSyncProfileStoresSummonerAndRanks(t *testing.T) {
	client := &fakeRiot{
		account:  riot.AccountDTO{PUUID: "puuid-1", GameName: "Test", TagLine: "EUW"},
		summoner: riot.SummonerDTO{PUUID: "puuid-1", SummonerLevel: 412, ProfileIconID: 5678},
		entries: []riot.LeagueEntryDTO{
			{QueueType: "RANKED_SOLO_5x5", Tier: "GOLD", Rank: "II", LeaguePoints: 47, Wins: 63, Losses: 58},
		},
	}
	repos := newFakeRepos()

	summoner, created, err := newTestService(client, repos).SyncProfile(context.Background(), testRegion, "Test", "EUW")
	require.NoError(t, err)
	assert.True(t, created, "саммонер добавлен впервые")

	assert.Equal(t, "puuid-1", summoner.PUUID)
	assert.Equal(t, "Test", summoner.RiotID)
	assert.Equal(t, testRegion, summoner.Region, "регион берётся от вызывающего: Riot его не возвращает")
	assert.Equal(t, 412, summoner.SummonerLevel)

	require.Contains(t, repos.summoners, "puuid-1")
	require.Len(t, repos.ranked["puuid-1"], 1)
	assert.Equal(t, "GOLD", repos.ranked["puuid-1"][0].Tier)
	assert.Equal(t, testNow, repos.ranked["puuid-1"][0].UpdatedAt)
}

// Профиль — то, ради чего пользователь пришёл; сбой League-V4 не должен ломать
// добавление саммонера.
func TestSyncProfileSurvivesLeagueFailure(t *testing.T) {
	client := &fakeRiot{
		account:   riot.AccountDTO{PUUID: "puuid-1", GameName: "Test", TagLine: "EUW"},
		summoner:  riot.SummonerDTO{PUUID: "puuid-1", SummonerLevel: 412},
		leagueErr: errors.New("riot прилёг"),
	}
	repos := newFakeRepos()

	_, _, err := newTestService(client, repos).SyncProfile(context.Background(), testRegion, "Test", "EUW")
	require.NoError(t, err)

	assert.Contains(t, repos.summoners, "puuid-1")
	assert.NotContains(t, repos.ranked, "puuid-1")
}

func TestSyncProfilePropagatesRiotErrors(t *testing.T) {
	repos := newFakeRepos()

	t.Run("аккаунт не найден", func(t *testing.T) {
		client := &fakeRiot{accountErr: riot.ErrNotFound}

		_, _, err := newTestService(client, repos).SyncProfile(context.Background(), testRegion, "Nobody", "XXX")
		assert.ErrorIs(t, err, riot.ErrNotFound)
	})

	t.Run("протухший ключ", func(t *testing.T) {
		client := &fakeRiot{profileErr: riot.ErrUnauthorized}

		_, _, err := newTestService(client, repos).SyncProfile(context.Background(), testRegion, "Test", "EUW")
		assert.ErrorIs(t, err, riot.ErrUnauthorized)
	})
}

func TestSyncMatchesSkipsAlreadyStored(t *testing.T) {
	client := &fakeRiot{
		matchIDs: []string{"EUW1_1", "EUW1_2", "EUW1_3"},
		matches: map[string]riot.MatchDetail{
			"EUW1_2": matchDetail("EUW1_2", "puuid-1"),
			"EUW1_3": matchDetail("EUW1_3", "puuid-1"),
		},
	}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}
	repos.matches["EUW1_1"] = domain.Match{MatchID: "EUW1_1"} // уже сохранён

	synced, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	require.NoError(t, err)

	assert.Equal(t, 2, synced)
	assert.Equal(t, []string{"EUW1_2", "EUW1_3"}, client.Requested(),
		"за уже сохранёнными матчами в Riot ходить незачем")
	assert.Equal(t, testNow, repos.syncedAt["puuid-1"])
}

func TestSyncMatchesKeepsOnlyTrackedParticipants(t *testing.T) {
	client := &fakeRiot{
		matchIDs: []string{"EUW1_1"},
		matches: map[string]riot.MatchDetail{
			"EUW1_1": matchDetail("EUW1_1", "puuid-1", "чужой-1", "puuid-2", "чужой-2"),
		},
	}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}
	repos.summoners["puuid-2"] = domain.Summoner{PUUID: "puuid-2", Region: testRegion}

	_, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	require.NoError(t, err)

	stored := repos.inserted["EUW1_1"]
	require.Len(t, stored, 2, "оба отслеживаемых участника матча попадают в таблицу")

	puuids := []string{stored[0].PUUID, stored[1].PUUID}
	assert.ElementsMatch(t, []string{"puuid-1", "puuid-2"}, puuids)
}

func TestSyncMatchesContinuesAfterSingleFailure(t *testing.T) {
	client := &fakeRiot{
		matchIDs: []string{"EUW1_1", "EUW1_2", "EUW1_3"},
		matches: map[string]riot.MatchDetail{
			"EUW1_1": matchDetail("EUW1_1", "puuid-1"),
			"EUW1_3": matchDetail("EUW1_3", "puuid-1"),
		},
		matchErrs: map[string]error{"EUW1_2": errors.New("riot прилёг")},
	}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}

	synced, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	require.NoError(t, err, "один сбойный матч не должен ронять весь прогон")

	assert.Equal(t, 2, synced)
	assert.Contains(t, repos.matches, "EUW1_1")
	assert.Contains(t, repos.matches, "EUW1_3")
	assert.NotContains(t, repos.matches, "EUW1_2")
	assert.Contains(t, repos.syncedAt, "puuid-1", "частичный прогресс — тоже прогресс")
}

// Протухший ключ не лечится следующим матчем: 20 бессмысленных запросов только
// сожгут лимит.
func TestSyncMatchesAbortsOnExpiredKey(t *testing.T) {
	client := &fakeRiot{
		matchIDs:  []string{"EUW1_1", "EUW1_2", "EUW1_3"},
		matchErrs: map[string]error{"EUW1_1": riot.ErrUnauthorized},
	}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}

	synced, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	assert.ErrorIs(t, err, riot.ErrUnauthorized)
	assert.Zero(t, synced)
	assert.Len(t, client.Requested(), 1, "после протухшего ключа продолжать незачем")
	assert.NotContains(t, repos.syncedAt, "puuid-1")
}

func TestSyncMatchesWithNothingNew(t *testing.T) {
	client := &fakeRiot{matchIDs: []string{"EUW1_1"}}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}
	repos.matches["EUW1_1"] = domain.Match{MatchID: "EUW1_1"}

	synced, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	require.NoError(t, err)

	assert.Zero(t, synced)
	assert.Empty(t, client.Requested())
	assert.Contains(t, repos.syncedAt, "puuid-1", "отметка обновляется даже без новых матчей")
}

func TestSyncMatchesPropagatesListError(t *testing.T) {
	client := &fakeRiot{idsErr: riot.ErrNotFound}

	repos := newFakeRepos()
	repos.summoners["puuid-1"] = domain.Summoner{PUUID: "puuid-1", Region: testRegion}

	_, err := newTestService(client, repos).SyncSummoner(context.Background(), "puuid-1", 10)
	assert.ErrorIs(t, err, riot.ErrNotFound)
	assert.NotContains(t, repos.syncedAt, "puuid-1")
}
