package riot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// Ограничения Match-V5 на пагинацию списка match id.
const (
	// MaxMatchIDCount — максимум, который принимает Riot за один запрос.
	MaxMatchIDCount = 100

	// DefaultMatchIDCount — размер страницы по умолчанию (SPEC.md 3.2).
	DefaultMatchIDCount = 20
)

// ErrEmptyMatchID возвращается при пустом идентификаторе матча.
var ErrEmptyMatchID = errors.New("riot: matchId обязателен")

// GetMatchIDsByPUUID возвращает идентификаторы матчей саммонера, свежие первыми.
//
// Match-V5 живёт на regional routing'е (SPEC.md 3.2).
func (c *Client) GetMatchIDsByPUUID(ctx context.Context, region, puuid string, start, count int) ([]string, error) {
	if puuid == "" {
		return nil, ErrEmptyPUUID
	}

	if start < 0 {
		return nil, fmt.Errorf("riot: start не может быть отрицательным, получено %d", start)
	}

	if count <= 0 || count > MaxMatchIDCount {
		return nil, fmt.Errorf("riot: count должен быть в диапазоне 1..%d, получено %d", MaxMatchIDCount, count)
	}

	route, err := MatchRoute(region)
	if err != nil {
		return nil, err
	}

	path := "/lol/match/v5/matches/by-puuid/" + url.PathEscape(puuid) + "/ids"
	query := url.Values{
		"start": []string{strconv.Itoa(start)},
		"count": []string{strconv.Itoa(count)},
	}

	var ids []string
	if _, err := c.get(ctx, route.Host(), path, query, &ids); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetMatch возвращает детали матча вместе с исходным JSON.
//
// Сырое тело кладётся в matches.raw_data (CLAUDE.md, отклонение 1), поэтому
// возвращается всегда, а не только когда о нём попросили.
func (c *Client) GetMatch(ctx context.Context, region, matchID string) (*MatchDetail, error) {
	if matchID == "" {
		return nil, ErrEmptyMatchID
	}

	route, err := MatchRoute(region)
	if err != nil {
		return nil, err
	}

	path := "/lol/match/v5/matches/" + url.PathEscape(matchID)

	var match MatchDTO

	raw, err := c.get(ctx, route.Host(), path, nil, &match)
	if err != nil {
		return nil, err
	}

	return &MatchDetail{Match: match, Raw: raw}, nil
}
