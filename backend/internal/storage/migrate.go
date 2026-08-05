package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	// Регистрирует database/sql-драйвер "pgx/v5", которым пользуется golang-migrate.
	_ "github.com/jackc/pgx/v5/stdlib"
)

var migrationsFS embed.FS

func Migrate(dsn string, logger *slog.Logger) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("не удалось прочитать вшитые миграции: %w", err)
	}
	defer func() { _ = source.Close() }()

	// sql.Open + WithInstance вместо migrate.New(dsn): pgx принимает и URL-форму
	// DSN, и keyword/value, а migrate.New потребовал бы схему pgx5:// в URL.
	db, err := sql.Open("pgx/v5", dsn)
	if err != nil {
		return fmt.Errorf("не удалось открыть соединение для миграций: %w", err)
	}
	defer func() { _ = db.Close() }()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("не удалось инициализировать драйвер миграций: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать migrate: %w", err)
	}

	err = m.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		logger.Info("миграции: изменений нет")
	case err != nil:
		return fmt.Errorf("не удалось применить миграции: %w", err)
	default:
		logger.Info("миграции применены")
	}

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("не удалось прочитать версию схемы: %w", err)
	}

	if dirty {
		return fmt.Errorf("схема БД в состоянии dirty на версии %d - требуется ручное вмешательство", version)
	}

	logger.Info("схема БД готова", "version", version)

	return nil
}
