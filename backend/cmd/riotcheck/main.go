// Команда riotcheck — ручная проверка клиента Riot API «вживую».
//
// Прогоняет все пять эндпоинтов из SPEC.md 3.2 на реальном ключе и печатает
// результат. Нужна прежде всего чтобы быстро отличить протухший ключ (он живёт
// 24 часа) от ошибки в коде.
//
//	go run ./cmd/riotcheck -region ru -riot-id "GameName#TAG"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/stacktraceo/league-companion/backend/internal/config"
	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

const (
	defaultRegion  = "ru"
	defaultCount   = 5
	defaultTimeout = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nОШИБКА: %v\n", err)

		if errors.Is(err, riot.ErrUnauthorized) {
			fmt.Fprintln(os.Stderr,
				"Похоже, RIOT_API_KEY протух — перевыпусти его на https://developer.riotgames.com "+
					"и обнови .env (ключ действует 24 часа).")
		}

		os.Exit(1)
	}
}

func run() error {
	region := flag.String("region", defaultRegion,
		"platform-регион: "+strings.Join(riot.SupportedRegions(), ", "))
	riotID := flag.String("riot-id", "", "Riot ID в формате GameName#TagLine (обязательно)")
	count := flag.Int("count", defaultCount, "сколько match id запросить (1..100)")
	matchID := flag.String("match", "", "конкретный matchId; по умолчанию — первый из списка")
	timeout := flag.Duration("timeout", defaultTimeout, "общий таймаут прогона")
	verbose := flag.Bool("v", false, "подробные логи запросов")
	flag.Parse()

	if *riotID == "" {
		flag.Usage()

		return errors.New("укажи -riot-id")
	}

	gameName, tagLine, err := splitRiotID(*riotID)
	if err != nil {
		return err
	}

	// .env локально; в CI/докере переменные приходят из окружения.
	if _, err := config.LoadDotEnv(); err != nil {
		return err
	}

	apiKey := strings.TrimSpace(os.Getenv("RIOT_API_KEY"))
	if apiKey == "" {
		return errors.New("RIOT_API_KEY не задан (положи его в .env — в корне репозитория или в backend/)")
	}

	logLevel := slog.LevelWarn
	if *verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	client := riot.New(apiKey, riot.WithLogger(logger), riot.WithTimeout(*timeout))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	return check(ctx, client, *region, gameName, tagLine, *count, *matchID)
}

func check(
	ctx context.Context,
	client *riot.Client,
	region, gameName, tagLine string,
	count int,
	matchID string,
) error {
	out := os.Stdout

	fmt.Fprintf(out, "Регион: %s\n", region)
	fmt.Fprintf(out, "Riot ID: %s#%s\n\n", gameName, tagLine)

	// 1. Account-V1 — regional routing.
	account, err := client.GetAccountByRiotID(ctx, region, gameName, tagLine)
	if err != nil {
		return fmt.Errorf("account-v1: %w", err)
	}

	fmt.Fprintf(out, "[1/5] account-v1     → puuid %s\n", account.PUUID)

	// 2. Summoner-V4 — platform routing.
	summoner, err := client.GetSummonerByPUUID(ctx, region, account.PUUID)
	if err != nil {
		return fmt.Errorf("summoner-v4: %w", err)
	}

	fmt.Fprintf(out, "[2/5] summoner-v4    → уровень %d, иконка %d\n",
		summoner.SummonerLevel, summoner.ProfileIconID)

	// 3. League-V4 — platform routing.
	entries, err := client.GetLeagueEntriesByPUUID(ctx, region, account.PUUID)
	if err != nil {
		return fmt.Errorf("league-v4: %w", err)
	}

	printRanks(out, entries)

	// 4. Match-V5, список id — regional routing.
	ids, err := client.GetMatchIDsByPUUID(ctx, region, account.PUUID, 0, count)
	if err != nil {
		return fmt.Errorf("match-v5 (ids): %w", err)
	}

	fmt.Fprintf(out, "[4/5] match-v5 ids   → %d шт.\n", len(ids))

	for _, id := range ids {
		fmt.Fprintf(out, "                       %s\n", id)
	}

	if matchID == "" {
		if len(ids) == 0 {
			fmt.Fprintln(out, "[5/5] match-v5       → пропущено: матчей нет")

			return nil
		}

		matchID = ids[0]
	}

	// 5. Match-V5, детали матча — regional routing.
	detail, err := client.GetMatch(ctx, region, matchID)
	if err != nil {
		return fmt.Errorf("match-v5 (%s): %w", matchID, err)
	}

	return printMatch(out, detail, account.PUUID)
}

func printRanks(out io.Writer, entries []riot.LeagueEntryDTO) {
	if len(entries) == 0 {
		fmt.Fprintln(out, "[3/5] league-v4      → без ранга")

		return
	}

	fmt.Fprintf(out, "[3/5] league-v4      → %d очередей\n", len(entries))

	for _, entry := range entries {
		fmt.Fprintf(out, "                       %s: %s %s, %d LP (%dW/%dL)\n",
			entry.QueueType, entry.Tier, entry.Rank, entry.LeaguePoints, entry.Wins, entry.Losses)
	}
}

func printMatch(out io.Writer, detail *riot.MatchDetail, puuid string) error {
	match, err := domain.MatchFromRiot(*detail)
	if err != nil {
		return err
	}

	participants, err := domain.MatchParticipantsFromRiot(*detail)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "[5/5] match-v5       → %s\n", match.MatchID)
	fmt.Fprintf(out, "                       начало %s, длительность %s, queue %d, патч %s\n",
		match.GameCreation.Format(time.RFC3339), match.GameDuration, match.QueueID, match.GameVersion)
	fmt.Fprintf(out, "                       сырой JSON: %d байт (пойдёт в matches.raw_data)\n",
		len(match.RawData))

	for _, p := range participants {
		marker := " "
		if p.PUUID == puuid {
			marker = ">"
		}

		result := "поражение"
		if p.Win {
			result = "победа"
		}

		fmt.Fprintf(out, "                     %s %-14s %2d/%2d/%2d  KDA %.2f  CS %3d  золото %5d  %s\n",
			marker, p.ChampionName, p.Kills, p.Deaths, p.Assists, p.KDA(), p.CS, p.GoldEarned, result)
	}

	return nil
}

// splitRiotID разбирает GameName#TagLine. Режем по последнему «#»: символ
// запрещён в игровых именах, но так надёжнее.
func splitRiotID(riotID string) (gameName, tagLine string, err error) {
	idx := strings.LastIndex(riotID, "#")
	if idx <= 0 || idx == len(riotID)-1 {
		return "", "", fmt.Errorf("некорректный Riot ID %q, ожидается GameName#TagLine", riotID)
	}

	return riotID[:idx], riotID[idx+1:], nil
}
