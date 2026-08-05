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

	// TTL короткий: sync worker вехи 7 ходит сюда именно за свежими матчами.
	req := request{host: route.Host(), path: path, query: query, ttl: matchIDsTTL, out: &ids}
	if _, err := c.do(ctx, req); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetMatch возвращает детали матча вместе с исходным JSON.
//
// Сырое тело кладётся в matches.raw_data (DECISIONS.md, отклонение 1), поэтому
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

	// matchTTL = 0: матч неизменяем и целиком ложится в matches.raw_data —
	// кэшировать сотни килобайт ещё и в Redis незачем.
	req := request{host: route.Host(), path: path, ttl: matchTTL, out: &match}

	raw, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	return &MatchDetail{Match: match, Raw: raw}, nil
}
