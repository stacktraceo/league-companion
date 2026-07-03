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

	"github.com/stacktraceo/league-companion/backend/internal/config"
	"github.com/stacktraceo/league-companion/backend/internal/httpapi"
	"github.com/stacktraceo/league-companion/backend/internal/storage"
)

// Таймауты HTTP-сервера — защита от медленных клиентов.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
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

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           httpapi.NewRouter(logger, pool),
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

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("не удалось корректно остановить сервер: %w", err)
	}

	logger.Info("сервер остановлен")

	return nil
}
