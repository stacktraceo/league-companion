// Package config читает конфигурацию сервиса из переменных окружения.
//
// Секреты (RIOT_API_KEY, CLIENT_API_KEY, пароль в DATABASE_URL) не попадают
// в логи: Config реализует slog.LogValuer и редактирует их (CLAUDE.md, «Конвенции»).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Дефолты для необязательных переменных.
const (
	defaultRedisAddr       = "localhost:6379"
	defaultHTTPPort        = 8080
	defaultLogLevel        = "info"
	defaultRiotHTTPTimeout = 10 * time.Second
	defaultSyncInterval    = 10 * time.Minute
)

const redacted = "[REDACTED]"

// Config — конфигурация бэкенда. Заполняется только через Load.
type Config struct {
	// RiotAPIKey — Development Key с developer.riotgames.com.
	// Протухает раз в 24 часа и обновляется вручную (SPEC.md 3.2) — поэтому
	// живёт в окружении, а не в сборке.
	RiotAPIKey string

	// RiotHTTPTimeout — таймаут одного запроса к Riot API.
	RiotHTTPTimeout time.Duration

	// DatabaseURL — DSN PostgreSQL (postgres://user:pass@host:port/db).
	DatabaseURL string

	// RedisAddr — адрес Redis для кэша.
	RedisAddr string

	// HTTPPort — порт, на котором слушает REST API.
	HTTPPort int

	// ClientAPIKey — shared secret между Android-клиентом и бэкендом,
	// проверяется middleware в заголовке X-API-Key (CLAUDE.md, отклонение 3).
	ClientAPIKey string

	// SyncInterval — как часто просыпается фоновая синхронизация (SPEC.md 3.5).
	// Ноль выключает её совсем: удобно для локальной отладки, когда лимит ключа
	// не хочется тратить на фон.
	SyncInterval time.Duration

	// LogLevel — минимальный уровень структурных логов.
	LogLevel slog.Level
}

// Load читает и валидирует конфигурацию из окружения.
//
// Все проблемы собираются в одну ошибку: при первом запуске полезнее увидеть
// сразу весь список незаданных переменных, чем чинить их по одной.
func Load() (*Config, error) {
	var errs []error

	cfg := &Config{
		RiotAPIKey:   os.Getenv("RIOT_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisAddr:    envOrDefault("REDIS_ADDR", defaultRedisAddr),
		ClientAPIKey: os.Getenv("CLIENT_API_KEY"),
	}

	if cfg.RiotAPIKey == "" {
		errs = append(errs, errors.New("RIOT_API_KEY обязателен (ключ с developer.riotgames.com)"))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL обязателен (postgres://user:pass@host:5432/db)"))
	}

	if cfg.ClientAPIKey == "" {
		errs = append(errs, errors.New("CLIENT_API_KEY обязателен (shared secret для заголовка X-API-Key)"))
	}

	port, err := parsePort(envOrDefault("HTTP_PORT", strconv.Itoa(defaultHTTPPort)))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.HTTPPort = port

	level, err := parseLogLevel(envOrDefault("LOG_LEVEL", defaultLogLevel))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.LogLevel = level

	timeout, err := parseTimeout(envOrDefault("RIOT_HTTP_TIMEOUT", defaultRiotHTTPTimeout.String()))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.RiotHTTPTimeout = timeout

	interval, err := parseSyncInterval(envOrDefault("SYNC_INTERVAL", defaultSyncInterval.String()))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.SyncInterval = interval

	if len(errs) > 0 {
		return nil, fmt.Errorf("некорректная конфигурация: %w", errors.Join(errs...))
	}

	return cfg, nil
}

// Addr возвращает адрес для http.Server.
func (c *Config) Addr() string {
	return ":" + strconv.Itoa(c.HTTPPort)
}

// LogValue реализует slog.LogValuer — секреты в логи не попадают.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("riot_api_key", maskSecret(c.RiotAPIKey)),
		slog.Duration("riot_http_timeout", c.RiotHTTPTimeout),
		slog.String("database_url", redactDSN(c.DatabaseURL)),
		slog.String("redis_addr", c.RedisAddr),
		slog.Int("http_port", c.HTTPPort),
		slog.String("client_api_key", maskSecret(c.ClientAPIKey)),
		slog.Duration("sync_interval", c.SyncInterval),
		slog.String("log_level", c.LogLevel.String()),
	)
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}

	return fallback
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("HTTP_PORT: %q не число", raw)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("HTTP_PORT: %d вне диапазона 1..65535", port)
	}

	return port, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL: %q не входит в debug|info|warn|error", raw)
	}
}

func parseTimeout(raw string) (time.Duration, error) {
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("RIOT_HTTP_TIMEOUT: %q не длительность (например 10s)", raw)
	}

	if timeout <= 0 {
		return 0, fmt.Errorf("RIOT_HTTP_TIMEOUT: %s должен быть положительным", timeout)
	}

	return timeout, nil
}

// parseSyncInterval разбирает интервал фоновой синхронизации. Ноль — валидное
// значение и означает «не синхронизировать в фоне»; отрицательное — ошибка.
func parseSyncInterval(raw string) (time.Duration, error) {
	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("SYNC_INTERVAL: %q не длительность (например 10m, 0 — выключить)", raw)
	}

	if interval < 0 {
		return 0, fmt.Errorf("SYNC_INTERVAL: %s не может быть отрицательным", interval)
	}

	return interval, nil
}

// maskSecret скрывает значение целиком, оставляя только признак «задано/не задано».
func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	return redacted
}

// redactDSN убирает пароль из DSN, сохраняя остальное — по нему удобно понять,
// к какой базе подключился сервис.
func redactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}

	u, err := url.Parse(dsn)
	if err != nil {
		// Распарсить не вышло — безопаснее скрыть строку целиком, чем рискнуть
		// напечатать в лог пароль.
		return redacted
	}

	return u.Redacted()
}
