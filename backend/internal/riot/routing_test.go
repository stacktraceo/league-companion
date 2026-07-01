package riot

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Таблица — единственный источник истины для теста: platform-хост, regional-маршрут
// для Match-V5 и regional-маршрут для Account-V1. См. SPEC.md 3.2 и риск в SPEC.md 7.
var routingCases = []struct {
	region       string
	platformHost string
	matchRoute   RegionalRoute
	accountRoute RegionalRoute
}{
	// Americas
	{"na1", "na1.api.riotgames.com", RouteAmericas, RouteAmericas},
	{"br1", "br1.api.riotgames.com", RouteAmericas, RouteAmericas},
	{"la1", "la1.api.riotgames.com", RouteAmericas, RouteAmericas},
	{"la2", "la2.api.riotgames.com", RouteAmericas, RouteAmericas},

	// Europe
	{"euw1", "euw1.api.riotgames.com", RouteEurope, RouteEurope},
	{"eun1", "eun1.api.riotgames.com", RouteEurope, RouteEurope},
	{"ru", "ru.api.riotgames.com", RouteEurope, RouteEurope},
	{"tr1", "tr1.api.riotgames.com", RouteEurope, RouteEurope},
	{"me1", "me1.api.riotgames.com", RouteEurope, RouteEurope},

	// Asia
	{"kr", "kr.api.riotgames.com", RouteAsia, RouteAsia},
	{"jp1", "jp1.api.riotgames.com", RouteAsia, RouteAsia},

	// SEA: Match-V5 обслуживается на sea, Account-V1 — нет, для него уходим на asia.
	{"oc1", "oc1.api.riotgames.com", RouteSEA, RouteAsia},
	{"sg2", "sg2.api.riotgames.com", RouteSEA, RouteAsia},
	{"ph2", "ph2.api.riotgames.com", RouteSEA, RouteAsia},
	{"th2", "th2.api.riotgames.com", RouteSEA, RouteAsia},
	{"tw2", "tw2.api.riotgames.com", RouteSEA, RouteAsia},
	{"vn2", "vn2.api.riotgames.com", RouteSEA, RouteAsia},
}

func TestPlatformHost(t *testing.T) {
	for _, tc := range routingCases {
		t.Run(tc.region, func(t *testing.T) {
			host, err := PlatformHost(tc.region)
			require.NoError(t, err)
			assert.Equal(t, tc.platformHost, host)
		})
	}
}

func TestMatchRoute(t *testing.T) {
	for _, tc := range routingCases {
		t.Run(tc.region, func(t *testing.T) {
			route, err := MatchRoute(tc.region)
			require.NoError(t, err)
			assert.Equal(t, tc.matchRoute, route)
		})
	}
}

func TestAccountRoute(t *testing.T) {
	for _, tc := range routingCases {
		t.Run(tc.region, func(t *testing.T) {
			route, err := AccountRoute(tc.region)
			require.NoError(t, err)
			assert.Equal(t, tc.accountRoute, route)
		})
	}
}

// Match- и Account-маршруты расходятся только на SEA-платформах. Тест фиксирует это
// расхождение явно, чтобы «упрощение» до одной функции не прошло незамеченным.
func TestSEAPlatformsRouteAccountToAsia(t *testing.T) {
	for _, region := range []string{"oc1", "sg2", "ph2", "th2", "tw2", "vn2"} {
		t.Run(region, func(t *testing.T) {
			matchRoute, err := MatchRoute(region)
			require.NoError(t, err)
			accountRoute, err := AccountRoute(region)
			require.NoError(t, err)

			assert.Equal(t, RouteSEA, matchRoute)
			assert.Equal(t, RouteAsia, accountRoute)
			assert.NotEqual(t, matchRoute, accountRoute)
		})
	}
}

func TestRoutingIsCaseAndSpaceInsensitive(t *testing.T) {
	for _, region := range []string{"RU", "Ru", " ru ", "\tRU\n"} {
		t.Run(region, func(t *testing.T) {
			host, err := PlatformHost(region)
			require.NoError(t, err)
			assert.Equal(t, "ru.api.riotgames.com", host)

			route, err := MatchRoute(region)
			require.NoError(t, err)
			assert.Equal(t, RouteEurope, route)
		})
	}
}

func TestUnknownRegion(t *testing.T) {
	// «europe»/«americas» — regional-маршруты, а не платформы. Подстановка одного
	// вместо другого — основной риск SPEC.md 7, она обязана падать, а не молча
	// возвращать пустую строку.
	unknown := []string{"", "europe", "americas", "asia", "sea", "eu", "euw", "na", "xx1", "ru1"}

	for _, region := range unknown {
		t.Run("region="+region, func(t *testing.T) {
			_, err := PlatformHost(region)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownRegion)
			assert.Contains(t, err.Error(), region)

			_, err = MatchRoute(region)
			assert.ErrorIs(t, err, ErrUnknownRegion)

			_, err = AccountRoute(region)
			assert.ErrorIs(t, err, ErrUnknownRegion)
		})
	}
}

func TestRegionalRouteHost(t *testing.T) {
	assert.Equal(t, "americas.api.riotgames.com", RouteAmericas.Host())
	assert.Equal(t, "asia.api.riotgames.com", RouteAsia.Host())
	assert.Equal(t, "europe.api.riotgames.com", RouteEurope.Host())
	assert.Equal(t, "sea.api.riotgames.com", RouteSEA.Host())
}

func TestSupportedRegions(t *testing.T) {
	regions := SupportedRegions()
	require.Len(t, regions, len(routingCases))

	// Отсортирован и без дублей — список идёт в help CLI и в тексты ошибок.
	for i := 1; i < len(regions); i++ {
		assert.Less(t, regions[i-1], regions[i], "SupportedRegions должен быть отсортирован")
	}

	// Каждый заявленный регион действительно резолвится.
	for _, region := range regions {
		_, err := PlatformHost(region)
		assert.NoError(t, err, "регион %q заявлен, но не резолвится", region)
	}
}

func TestErrUnknownRegionWrapping(t *testing.T) {
	_, err := MatchRoute("bogus")
	require.Error(t, err)

	var target error = ErrUnknownRegion
	assert.True(t, errors.Is(err, target))
}
