// Команда server — HTTP-бэкенд League Companion.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stacktraceo/league-companion/backend/internal/cache"
	"github.com/stacktraceo/league-companion/backend/internal/config"
	"github.com/stacktraceo/league-companion/backend/internal/httpapi"
	"github.com/stacktraceo/league-companion/backend/internal/ratelimit"
	"github.com/stacktraceo/league-companion/backend/internal/riot"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
	"github.com/stacktraceo/league-companion/backend/internal/syncer"
)

// Таймауты HTTP-сервера — защита от медленных клиентов.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second

	// syncShutdownTimeout — сколько ждать активные фоновые синхронизации.
	// Меньше shutdownTimeout: сначала дожидаемся их, потом закрываем HTTP.
	syncShutdownTimeout = 8 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервер остановлен с ошибкой", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// .env удобен локально; в docker-compose переменные приходят из окружения,
	// поэтому его отсутствие — не ошибка.
	dotEnvFiles, err := config.LoadDotEnv()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	logger.Info("конфигурация загружена", "config", cfg, "dotenv", dotEnvFiles)

	// Миграции до подключения пула: если схема не поднялась, поднимать сервис
	// смысла нет.
	if err := storage.Migrate(cfg.DatabaseURL, logger); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := storage.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Один лимитер на процесс: лимиты Riot глобальны на ключ (SPEC.md 3.2),
	// поэтому HTTP-хендлеры и фоновые воркеры стоят в одной очереди. Отдельный
	// лимитер на каждого означал бы двойной расход и 429 от Riot.
	limiter := ratelimit.New(ratelimit.RiotDevKeyLimits...)

	// Недоступный Redis не должен ронять сервис — Open деградирует в кэш в памяти.
	responseCache := cache.Open(ctx, cfg.RedisAddr, logger)
	defer func() {
		if err := responseCache.Close(); err != nil {
			logger.Warn("не удалось закрыть кэш", "error", err)
		}
	}()

	riotClient := riot.New(cfg.RiotAPIKey,
		riot.WithLogger(logger),
		riot.WithTimeout(cfg.RiotHTTPTimeout),
		riot.WithRateLimiter(limiter),
		riot.WithCache(responseCache),
	)

	summoners := storage.NewSummoners(pool)
	ranked := storage.NewRankedStats(pool)
	matches := storage.NewMatches(pool)

	syncService := syncer.NewService(riotClient, summoners, ranked, matches, logger)

	runner := syncer.NewRunner(syncService, syncer.DefaultWorkers, syncer.DefaultQueueSize, logger)
	// Базовый контекст — контекст приложения, а не запроса: HTTP-запрос
	// завершается сразу после ответа, а синхронизация продолжается в фоне.
	// WithoutCancel нужен потому, что ctx отменяется по сигналу остановки, а
	// активные задачи должны доживать до Shutdown ниже, а не обрываться сразу.
	runner.Start(context.WithoutCancel(ctx))

	// Зарегистрировано после defer pool.Close(), поэтому выполнится раньше него:
	// воркеры не должны обращаться к уже закрытому пулу.
	defer stopRunner(runner, logger)

	server := &http.Server{
		Addr: cfg.Addr(),
		Handler: httpapi.NewRouter(httpapi.Deps{
			Logger:       logger,
			DB:           pool,
			ClientAPIKey: cfg.ClientAPIKey,
			Profiles:     syncService,
			Queue:        runner,
			Summoners:    summoners,
			Ranked:       ranked,
			Matches:      matches,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		logger.Info("сервер слушает", "addr", server.Addr)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http-сервер упал: %w", err)

			return
		}

		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("получен сигнал остановки, завершаемся")
	}

	// Контекст сигнала уже отменён — для shutdown нужен свой.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Сначала HTTP: пока он принимает запросы, в очередь продолжают падать новые
	// задачи. Фоновые воркеры дожидаются в отложенном stopRunner.
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("не удалось корректно остановить сервер: %w", err)
	}

	logger.Info("сервер остановлен")

	return nil
}

// stopRunner дожидается активных фоновых синхронизаций.
//
// Ждём ограниченное время: один прогон — это десятки запросов к Riot под лимитом,
// и растягивать из-за него остановку сервиса неправильно. Обрыв безопасен: каждый
// матч сохраняется отдельной транзакцией, а недокачанные подберёт следующий прогон
// (KnownIDs отфильтрует уже сохранённые).
func stopRunner(runner *syncer.Runner, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), syncShutdownTimeout)
	defer cancel()

	if err := runner.Shutdown(ctx); err != nil {
		logger.Warn("фоновые синхронизации прерваны по таймауту", "error", err)

		return
	}

	logger.Info("фоновая синхронизация остановлена")
}
