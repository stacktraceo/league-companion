package riot

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Riot использует два независимых вида роутинга, и подстановка одного вместо
// другого — самая частая ошибка интеграции (SPEC.md 3.2, риск в SPEC.md 7):
//
//   - platform routing (ru, euw1, na1, ...) — Summoner-V4, League-V4;
//   - regional routing (europe, americas, asia, sea) — Account-V1, Match-V5.
//
// Весь маппинг живёт только здесь и покрыт routing_test.go. Клиент не должен
// собирать хосты вручную.

// RegionalRoute — значение regional routing'а Riot API.
type RegionalRoute string

// Regional-маршруты, поддерживаемые Riot API.
const (
	RouteAmericas RegionalRoute = "americas"
	RouteAsia     RegionalRoute = "asia"
	RouteEurope   RegionalRoute = "europe"
	RouteSEA      RegionalRoute = "sea"
)

// ErrUnknownRegion возвращается, если регион не входит в таблицу платформ.
var ErrUnknownRegion = errors.New("unknown region")

// Host возвращает базовый хост regional-маршрута.
func (r RegionalRoute) Host() string {
	return string(r) + apiHostSuffix
}

const apiHostSuffix = ".api.riotgames.com"

// routing — маршруты одной платформы.
//
// Для SEA-платформ match и account расходятся: Match-V5 обслуживается на
// sea.api.riotgames.com, а Account-V1 там не публикуется — для него используется
// asia. Account-V1 отдаёт глобальные данные, поэтому любой валидный regional-хост
// вернёт одинаковый результат.
type routing struct {
	match   RegionalRoute
	account RegionalRoute
}

var platformRouting = map[string]routing{
	// Americas
	"br1": {match: RouteAmericas, account: RouteAmericas},
	"la1": {match: RouteAmericas, account: RouteAmericas},
	"la2": {match: RouteAmericas, account: RouteAmericas},
	"na1": {match: RouteAmericas, account: RouteAmericas},

	// Europe
	"eun1": {match: RouteEurope, account: RouteEurope},
	"euw1": {match: RouteEurope, account: RouteEurope},
	"me1":  {match: RouteEurope, account: RouteEurope},
	"ru":   {match: RouteEurope, account: RouteEurope},
	"tr1":  {match: RouteEurope, account: RouteEurope},

	// Asia
	"jp1": {match: RouteAsia, account: RouteAsia},
	"kr":  {match: RouteAsia, account: RouteAsia},

	// SEA
	"oc1": {match: RouteSEA, account: RouteAsia},
	"ph2": {match: RouteSEA, account: RouteAsia},
	"sg2": {match: RouteSEA, account: RouteAsia},
	"th2": {match: RouteSEA, account: RouteAsia},
	"tw2": {match: RouteSEA, account: RouteAsia},
	"vn2": {match: RouteSEA, account: RouteAsia},
}

// lookup нормализует регион и находит его маршруты.
func lookup(region string) (string, routing, error) {
	normalized := strings.ToLower(strings.TrimSpace(region))

	r, ok := platformRouting[normalized]
	if !ok {
		return "", routing{}, fmt.Errorf("%w: %q (поддерживаются: %s)",
			ErrUnknownRegion, region, strings.Join(SupportedRegions(), ", "))
	}

	return normalized, r, nil
}

// PlatformHost возвращает platform-хост региона — для Summoner-V4 и League-V4.
func PlatformHost(region string) (string, error) {
	normalized, _, err := lookup(region)
	if err != nil {
		return "", err
	}

	return normalized + apiHostSuffix, nil
}

// MatchRoute возвращает regional-маршрут региона для Match-V5.
func MatchRoute(region string) (RegionalRoute, error) {
	_, r, err := lookup(region)
	if err != nil {
		return "", err
	}

	return r.match, nil
}

// AccountRoute возвращает regional-маршрут региона для Account-V1.
func AccountRoute(region string) (RegionalRoute, error) {
	_, r, err := lookup(region)
	if err != nil {
		return "", err
	}

	return r.account, nil
}

// SupportedRegions возвращает отсортированный список поддерживаемых платформ —
// для help'а CLI и текстов ошибок.
func SupportedRegions() []string {
	regions := make([]string, 0, len(platformRouting))
	for region := range platformRouting {
		regions = append(regions, region)
	}
	sort.Strings(regions)

	return regions
}
