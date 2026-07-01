package riot

import (
	"context"
	"net/url"
)

// GetLeagueEntriesByPUUID возвращает ранговые записи саммонера по всем очередям.
//
// League-V4 живёт на platform routing'е (SPEC.md 3.2). У безранговых саммонеров
// список пустой — это не ошибка.
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
	if _, err := c.get(ctx, host, path, nil, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}
