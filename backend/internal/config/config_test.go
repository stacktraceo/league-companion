package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRiotKey   = "RGAPI-test-key-not-real"
	testClientKey = "test-client-secret"
	testDSN       = "postgres://lc:s3cret@localhost:5432/league_companion?sslmode=disable"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("RIOT_API_KEY", testRiotKey)
	t.Setenv("DATABASE_URL", testDSN)
	t.Setenv("CLIENT_API_KEY", testClientKey)
}

func clearOptional(t *testing.T) {
	t.Helper()
	for _, key := range []string{"REDIS_ADDR", "HTTP_PORT", "LOG_LEVEL", "RIOT_HTTP_TIMEOUT", "SYNC_INTERVAL"} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
	clearOptional(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, testRiotKey, cfg.RiotAPIKey)
	assert.Equal(t, testDSN, cfg.DatabaseURL)
	assert.Equal(t, testClientKey, cfg.ClientAPIKey)
	assert.Equal(t, defaultRedisAddr, cfg.RedisAddr)
	assert.Equal(t, defaultHTTPPort, cfg.HTTPPort)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Equal(t, defaultRiotHTTPTimeout, cfg.RiotHTTPTimeout)
	assert.Equal(t, defaultSyncInterval, cfg.SyncInterval)
	assert.Equal(t, ":8080", cfg.Addr())
}

func TestLoadOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("RIOT_HTTP_TIMEOUT", "3s")
	t.Setenv("SYNC_INTERVAL", "45s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "redis:6379", cfg.RedisAddr)
	assert.Equal(t, 9090, cfg.HTTPPort)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, 3*time.Second, cfg.RiotHTTPTimeout)
	assert.Equal(t, 45*time.Second, cfg.SyncInterval)
	assert.Equal(t, ":9090", cfg.Addr())
}

func TestLoadSyncIntervalZeroDisablesBackgroundSync(t *testing.T) {
	setRequired(t)
	clearOptional(t)
	t.Setenv("SYNC_INTERVAL", "0")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Zero(t, cfg.SyncInterval)
}

func TestLoadReportsAllMissingRequiredAtOnce(t *testing.T) {
	t.Setenv("RIOT_API_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CLIENT_API_KEY", "")
	clearOptional(t)

	_, err := Load()
	require.Error(t, err)

	// Все три проблемы - в одной ошибке, чинить по одной не приходится.
	assert.Contains(t, err.Error(), "RIOT_API_KEY")
	assert.Contains(t, err.Error(), "DATABASE_URL")
	assert.Contains(t, err.Error(), "CLIENT_API_KEY")
}

func TestLoadInvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{"порт не число", "HTTP_PORT", "abc", "HTTP_PORT"},
		{"порт вне диапазона", "HTTP_PORT", "70000", "70000"},
		{"нулевой порт", "HTTP_PORT", "0", "HTTP_PORT"},
		{"неизвестный уровень логов", "LOG_LEVEL", "verbose", "LOG_LEVEL"},
		{"таймаут не длительность", "RIOT_HTTP_TIMEOUT", "10", "RIOT_HTTP_TIMEOUT"},
		{"отрицательный таймаут", "RIOT_HTTP_TIMEOUT", "-5s", "RIOT_HTTP_TIMEOUT"},
		{"интервал не длительность", "SYNC_INTERVAL", "10", "SYNC_INTERVAL"},
		{"отрицательный интервал", "SYNC_INTERVAL", "-1m", "SYNC_INTERVAL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setRequired(t)
			clearOptional(t)
			t.Setenv(tc.key, tc.value)

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLogLevelAliases(t *testing.T) {
	for raw, want := range map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"Error":   slog.LevelError,
		" info ":  slog.LevelInfo,
	} {
		t.Run(raw, func(t *testing.T) {
			setRequired(t)
			clearOptional(t)
			t.Setenv("LOG_LEVEL", raw)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, want, cfg.LogLevel)
		})
	}
}

func TestLogValueRedactsSecrets(t *testing.T) {
	setRequired(t)
	clearOptional(t)

	cfg, err := Load()
	require.NoError(t, err)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("config loaded", "config", cfg)

	logged := buf.String()

	assert.NotContains(t, logged, testRiotKey, "RIOT_API_KEY утёк в лог")
	assert.NotContains(t, logged, testClientKey, "CLIENT_API_KEY утёк в лог")
	assert.NotContains(t, logged, "s3cret", "пароль из DATABASE_URL утёк в лог")

	// При этом полезная часть конфига в логе остаётся.
	assert.Contains(t, logged, "localhost:5432")
	assert.Contains(t, logged, "league_companion")
	assert.Contains(t, logged, redacted)
}

func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"пустой", "", ""},
		{"с паролем", "postgres://lc:s3cret@db:5432/lc", "postgres://lc:xxxxx@db:5432/lc"},
		{"без пароля", "postgres://db:5432/lc", "postgres://db:5432/lc"},
		{"нераспарсиваемый", "postgres://lc:s3cret@%%%/lc", redacted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactDSN(tc.dsn)
			assert.Equal(t, tc.want, got)
			if tc.dsn != "" {
				assert.False(t, strings.Contains(got, "s3cret"), "пароль остался в %q", got)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	assert.Equal(t, "", maskSecret(""))
	assert.Equal(t, redacted, maskSecret("RGAPI-whatever"))
}
