// Команда riotcheck — ручная проверка клиента Riot API «вживую».
//
// Прогоняет все пять эндпоинтов из SPEC.md 3.2 на реальном ключе и печатает
// результат. Нужна прежде всего чтобы быстро отличить протухший ключ (он живёт
// 24 часа) от ошибки в коде.
//
//	go run ./cmd/riotcheck -region ru -riot-id "GameName#TAG"
//
// С -repeat видно работу кэша и ограничителя: со второго прохода профиль, ранг и
// список матчей приходят из кэша, а детали матча каждый раз идут в Riot.
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

	"github.com/stacktraceo/league-companion/backend/internal/cache"
	"github.com/stacktraceo/league-companion/backend/internal/config"
	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/ratelimit"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
)

const (
	defaultRegion  = "ru"
	defaultCount   = 5
	defaultTimeout = time.Minute

	// memorySentinel в -redis заставляет работать на кэше в памяти, даже если
	// REDIS_ADDR задан.
	memorySentinel = "none"
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
	repeat := flag.Int("repeat", 1, "сколько раз прогнать проверку: со второго прохода видно кэш")
	redisAddr := flag.String("redis", "",
		`адрес Redis; пусто — взять REDIS_ADDR, "`+memorySentinel+`" — кэш в памяти процесса`)
	timeout := flag.Duration("timeout", defaultTimeout, "общий таймаут прогона")
	verbose := flag.Bool("v", false, "подробные логи запросов")
	flag.Parse()

	if *riotID == "" {
		flag.Usage()

		return errors.New("укажи -riot-id")
	}

	if *repeat < 1 {
		return fmt.Errorf("-repeat должен быть не меньше 1, получено %d", *repeat)
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	responseCache := cache.Open(ctx, resolveRedisAddr(*redisAddr), logger)
	defer func() { _ = responseCache.Close() }()

	// Лимитер один на процесс: лимиты Riot глобальны на ключ (SPEC.md 3.2).
	client := riot.New(apiKey,
		riot.WithLogger(logger),
		riot.WithRateLimiter(ratelimit.New(ratelimit.RiotDevKeyLimits...)),
		riot.WithCache(responseCache),
	)

	out := os.Stdout

	fmt.Fprintf(out, "Регион: %s\n", *region)
	fmt.Fprintf(out, "Riot ID: %s#%s\n", gameName, tagLine)
	fmt.Fprintf(out, "Кэш: %s\n", describeCache(responseCache))

	for pass := 1; pass <= *repeat; pass++ {
		if *repeat > 1 {
			fmt.Fprintf(out, "\n=== проход %d из %d ===\n", pass, *repeat)
		} else {
			fmt.Fprintln(out)
		}

		started := time.Now()

		if err := check(ctx, out, client, *region, gameName, tagLine, *count, *matchID, pass == 1); err != nil {
			return err
		}

		fmt.Fprintf(out, "проход занял %s\n", time.Since(started).Round(time.Millisecond))
	}

	return nil
}

// resolveRedisAddr выбирает адрес Redis: явный флаг, иначе REDIS_ADDR,
// а memorySentinel означает работу без Redis.
func resolveRedisAddr(flagValue string) string {
	switch flagValue {
	case "":
		return strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	case memorySentinel:
		return ""
	default:
		return flagValue
	}
}

func describeCache(c cache.Cache) string {
	if _, ok := c.(*cache.Redis); ok {
		return "redis"
	}

	return "в памяти процесса"
}

func check(
	ctx context.Context,
	out io.Writer,
	client *riot.Client,
	region, gameName, tagLine string,
	count int,
	matchID string,
	detailed bool,
) error {
	// 1. Account-V1 — regional routing.
	started := time.Now()

	account, err := client.GetAccountByRiotID(ctx, region, gameName, tagLine)
	if err != nil {
		return fmt.Errorf("account-v1: %w", err)
	}

	fmt.Fprintf(out, "[1/5] account-v1     → puuid %s%s\n", account.PUUID, took(started))

	// 2. Summoner-V4 — platform routing.
	started = time.Now()

	summoner, err := client.GetSummonerByPUUID(ctx, region, account.PUUID)
	if err != nil {
		return fmt.Errorf("summoner-v4: %w", err)
	}

	fmt.Fprintf(out, "[2/5] summoner-v4    → уровень %d, иконка %d%s\n",
		summoner.SummonerLevel, summoner.ProfileIconID, took(started))

	// 3. League-V4 — platform routing.
	started = time.Now()

	entries, err := client.GetLeagueEntriesByPUUID(ctx, region, account.PUUID)
	if err != nil {
		return fmt.Errorf("league-v4: %w", err)
	}

	printRanks(out, entries, took(started))

	// 4. Match-V5, список id — regional routing.
	started = time.Now()

	ids, err := client.GetMatchIDsByPUUID(ctx, region, account.PUUID, 0, count)
	if err != nil {
		return fmt.Errorf("match-v5 (ids): %w", err)
	}

	fmt.Fprintf(out, "[4/5] match-v5 ids   → %d шт.%s\n", len(ids), took(started))

	if detailed {
		for _, id := range ids {
			fmt.Fprintf(out, "                       %s\n", id)
		}
	}

	if matchID == "" {
		if len(ids) == 0 {
			fmt.Fprintln(out, "[5/5] match-v5       → пропущено: матчей нет")

			return nil
		}

		matchID = ids[0]
	}

	// 5. Match-V5, детали матча — regional routing.
	started = time.Now()

	detail, err := client.GetMatch(ctx, region, matchID)
	if err != nil {
		return fmt.Errorf("match-v5 (%s): %w", matchID, err)
	}

	return printMatch(out, detail, account.PUUID, took(started), detailed)
}

// took форматирует длительность шага — по ней видно попадания в кэш.
func took(started time.Time) string {
	return fmt.Sprintf("  [%s]", time.Since(started).Round(time.Millisecond))
}

func printRanks(out io.Writer, entries []riot.LeagueEntryDTO, took string) {
	if len(entries) == 0 {
		fmt.Fprintf(out, "[3/5] league-v4      → без ранга%s\n", took)

		return
	}

	fmt.Fprintf(out, "[3/5] league-v4      → %d очередей%s\n", len(entries), took)

	for _, entry := range entries {
		fmt.Fprintf(out, "                       %s: %s %s, %d LP (%dW/%dL)\n",
			entry.QueueType, entry.Tier, entry.Rank, entry.LeaguePoints, entry.Wins, entry.Losses)
	}
}

func printMatch(out io.Writer, detail *riot.MatchDetail, puuid, took string, detailed bool) error {
	match, err := domain.MatchFromRiot(*detail)
	if err != nil {
		return err
	}

	participants, err := domain.MatchParticipantsFromRiot(*detail)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "[5/5] match-v5       → %s%s\n", match.MatchID, took)

	if !detailed {
		return nil
	}

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
