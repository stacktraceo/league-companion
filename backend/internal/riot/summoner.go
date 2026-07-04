package riot

import (
	"context"
	"errors"
	"net/url"
)

// ErrEmptyPUUID возвращается при пустом PUUID.
var ErrEmptyPUUID = errors.New("riot: puuid обязателен")

// GetSummonerByPUUID возвращает профиль саммонера: уровень и иконку.
//
// Summoner-V4 живёт на platform routing'е (SPEC.md 3.2).
func (c *Client) GetSummonerByPUUID(ctx context.Context, region, puuid string) (*SummonerDTO, error) {
	if puuid == "" {
		return nil, ErrEmptyPUUID
	}

	host, err := PlatformHost(region)
	if err != nil {
		return nil, err
	}

	path := "/lol/summoner/v4/summoners/by-puuid/" + url.PathEscape(puuid)

	var summoner SummonerDTO

	req := request{host: host, path: path, ttl: summonerTTL, out: &summoner}
	if _, err := c.do(ctx, req); err != nil {
		return nil, err
	}

	return &summoner, nil
}
