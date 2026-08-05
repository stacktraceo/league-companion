package riot

import (
	"context"
	"errors"
	"net/url"
)

var ErrEmptyRiotID = errors.New("riot: gameName и tagLine обязательны")

func (c *Client) GetAccountByRiotID(ctx context.Context, region, gameName, tagLine string) (*AccountDTO, error) {
	if gameName == "" || tagLine == "" {
		return nil, ErrEmptyRiotID
	}

	route, err := AccountRoute(region)
	if err != nil {
		return nil, err
	}

	// Игровые имена содержат пробелы и юникод - экранируем как сегменты пути.
	path := "/riot/account/v1/accounts/by-riot-id/" + url.PathEscape(gameName) + "/" + url.PathEscape(tagLine)

	var account AccountDTO

	// Riot ID → PUUID меняется только при смене ника, поэтому TTL длинный.
	req := request{host: route.Host(), path: path, ttl: accountTTL, out: &account}
	if _, err := c.do(ctx, req); err != nil {
		return nil, err
	}

	return &account, nil
}
