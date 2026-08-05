package riot

import (
	"context"
	"net/url"
)

func (c *Client) GetLeagueEntriesByPUUID(ctx context.Context, region, puuid string) ([]LeagueEntryDTO, error) {
	if puuid == "" {
		return nil, ErrEmptyPUUID
	}

	host, err := PlatformHost(region)
	if err != nil {
		return nil, err
	}

	path := "/lol/league/v4/entries/by-puuid/" + url.PathEscape(puuid)

	var entries []LeagueEntryDTO

	// LP меняется после каждой игры - TTL короткий.
	req := request{host: host, path: path, ttl: leagueTTL, out: &entries}
	if _, err := c.do(ctx, req); err != nil {
		return nil, err
	}

	return entries, nil
}
