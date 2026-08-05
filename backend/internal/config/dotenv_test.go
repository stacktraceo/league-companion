package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chdirToNestedTempDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	nested := filepath.Join(root, "backend")
	require.NoError(t, os.Mkdir(nested, 0o755))
	t.Chdir(nested)

	return root
}

func unsetAfter(t *testing.T, names ...string) {
	t.Helper()

	t.Cleanup(func() {
		for _, name := range names {
			_ = os.Unsetenv(name)
		}
	})
}

func TestLoadDotEnvFindsRepoRoot(t *testing.T) {
	root := chdirToNestedTempDir(t)
	unsetAfter(t, "FROM_ROOT")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("FROM_ROOT=root-value\n"), 0o600))

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join("..", ".env")}, loaded)
	assert.Equal(t, "root-value", os.Getenv("FROM_ROOT"))
}

func TestLoadDotEnvPrefersNearestFile(t *testing.T) {
	root := chdirToNestedTempDir(t)
	unsetAfter(t, "SHARED", "ONLY_ROOT")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("SHARED=root\nONLY_ROOT=root\n"), 0o600))
	require.NoError(t, os.WriteFile(".env", []byte("SHARED=nested\n"), 0o600))

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	assert.Equal(t, []string{".env", filepath.Join("..", ".env")}, loaded)

	// Ближний файл выигрывает при совпадении ключа, дальний дополняет остальным.
	assert.Equal(t, "nested", os.Getenv("SHARED"))
	assert.Equal(t, "root", os.Getenv("ONLY_ROOT"))
}

func TestLoadDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	root := chdirToNestedTempDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("PRESET=from-file\n"), 0o600))
	t.Setenv("PRESET", "from-environment")

	_, err := LoadDotEnv()
	require.NoError(t, err)
	assert.Equal(t, "from-environment", os.Getenv("PRESET"))
}

func TestLoadDotEnvWithoutFilesIsNotAnError(t *testing.T) {
	chdirToNestedTempDir(t)

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	assert.Empty(t, loaded)
}
