package riot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCache — минимальный кэш в памяти без TTL: время жизни проверяется в
// пакете cache, здесь важно лишь то, ходит ли клиент в кэш и что он туда кладёт.
type fakeCache struct {
	mu     sync.Mutex
	items  map[string][]byte
	ttls   map[string]time.Duration
	getErr error
	setErr error
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		items: make(map[string][]byte),
		ttls:  make(map[string]time.Duration),
	}
}

func (c *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.getErr != nil {
		return nil, false, c.getErr
	}

	value, ok := c.items[key]

	return value, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.setErr != nil {
		return c.setErr
	}

	c.items[key] = append([]byte(nil), value...)
	c.ttls[key] = ttl

	return nil
}

func (c *fakeCache) keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}

	return keys
}

func (c *fakeCache) ttl(key string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ttls[key]
}

func TestCacheKeyIncludesHostPathAndQuery(t *testing.T) {
	base := request{host: "europe.api.riotgames.com", path: "/lol/match/v5/matches/by-puuid/p/ids"}

	assert.Equal(t, "europe.api.riotgames.com/lol/match/v5/matches/by-puuid/p/ids", base.cacheKey())

	withQuery := base
	withQuery.query = url.Values{"start": []string{"0"}, "count": []string{"20"}}
	assert.Equal(t,
		"europe.api.riotgames.com/lol/match/v5/matches/by-puuid/p/ids?count=20&start=0",
		withQuery.cacheKey())

	// Разные регионы не должны схлопываться в один ключ: путь у них одинаковый.
	other := base
	other.host = "americas.api.riotgames.com"
	assert.NotEqual(t, base.cacheKey(), other.cacheKey())

	// Ключ Riot в кэш не попадает ни при каких условиях (DECISIONS.md, «Конвенции»).
	assert.NotContains(t, withQuery.cacheKey(), testAPIKey)
}

func TestSecondCallIsServedFromCache(t *testing.T) {
	var calls atomic.Int64

	cache := newFakeCache()
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"puuid":"`+testPUUID+`","gameName":"Name","tagLine":"TAG"}`)
	}, WithCache(cache))

	ctx := context.Background()

	first, err := client.GetAccountByRiotID(ctx, "ru", "Name", "TAG")
	require.NoError(t, err)

	second, err := client.GetAccountByRiotID(ctx, "ru", "Name", "TAG")
	require.NoError(t, err)

	assert.Equal(t, int64(1), calls.Load(), "второй вызов не должен уходить в Riot")
	assert.Equal(t, first.PUUID, second.PUUID)
	assert.Equal(t, "Name", second.GameName)

	require.Len(t, cache.keys(), 1)
	assert.Equal(t, accountTTL, cache.ttl(cache.keys()[0]))
}

func TestCacheHitDoesNotSpendRateLimit(t *testing.T) {
	cache := newFakeCache()
	limiter := &countingWaiter{}

	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"puuid":"`+testPUUID+`"}`)
	}, WithCache(cache), WithRateLimiter(limiter))

	ctx := context.Background()

	_, err := client.GetSummonerByPUUID(ctx, "ru", testPUUID)
	require.NoError(t, err)

	_, err = client.GetSummonerByPUUID(ctx, "ru", testPUUID)
	require.NoError(t, err)

	assert.Equal(t, int64(1), limiter.calls.Load(),
		"бюджет Riot тратит только реальный запрос, а не попадание в кэш")
}

// Детали матча не кэшируются: они неизменяемы и целиком уходят в matches.raw_data.
func TestMatchDetailsAreNotCached(t *testing.T) {
	var calls atomic.Int64

	cache := newFakeCache()
	raw := fixture(t, "match.json")

	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, raw)
	}, WithCache(cache))

	ctx := context.Background()

	_, err := client.GetMatch(ctx, "euw1", "EUW1_7000000001")
	require.NoError(t, err)

	_, err = client.GetMatch(ctx, "euw1", "EUW1_7000000001")
	require.NoError(t, err)

	assert.Equal(t, int64(2), calls.Load())
	assert.Empty(t, cache.keys(), "matchTTL = 0 — кэш не трогаем вовсе")
}

func TestOnlySuccessfulResponsesAreCached(t *testing.T) {
	cache := newFakeCache()
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}, WithCache(cache))

	_, err := client.GetAccountByRiotID(context.Background(), "ru", "Nobody", "XXX")
	require.ErrorIs(t, err, ErrNotFound)

	assert.Empty(t, cache.keys(), "ошибки Riot кэшировать нельзя")
}

func TestBrokenCacheDoesNotBreakRequests(t *testing.T) {
	for name, prepare := range map[string]func(*fakeCache){
		"кэш не читается": func(c *fakeCache) { c.getErr = errors.New("redis недоступен") },
		"кэш не пишется":  func(c *fakeCache) { c.setErr = errors.New("redis недоступен") },
		"в кэше мусор": func(c *fakeCache) {
			c.items["ru.api.riotgames.com/lol/summoner/v4/summoners/by-puuid/"+testPUUID] = []byte("не json")
		},
	} {
		t.Run(name, func(t *testing.T) {
			cache := newFakeCache()
			prepare(cache)

			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"puuid":"`+testPUUID+`","summonerLevel":412}`)
			}, WithCache(cache))

			summoner, err := client.GetSummonerByPUUID(context.Background(), "ru", testPUUID)
			require.NoError(t, err)
			assert.Equal(t, int64(412), summoner.SummonerLevel)
		})
	}
}

func TestClientWithoutCacheAlwaysGoesToRiot(t *testing.T) {
	var calls atomic.Int64

	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"puuid":"`+testPUUID+`"}`)
	})

	ctx := context.Background()

	_, err := client.GetSummonerByPUUID(ctx, "ru", testPUUID)
	require.NoError(t, err)

	_, err = client.GetSummonerByPUUID(ctx, "ru", testPUUID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), calls.Load())
}
