package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// dotEnvCandidates — где искать .env, в порядке убывания приоритета:
// рядом с модулем (backend/.env) и в корне репозитория. Корень нужен потому,
// что docker-compose из вехи «День 8» подхватывает .env именно оттуда, и держать
// ключ Riot в двух местах не хочется.
var dotEnvCandidates = []string{
	".env",
	filepath.Join("..", ".env"),
}

// LoadDotEnv подгружает переменные из всех найденных .env и возвращает пути,
// откуда читал.
//
// Отсутствие файлов — не ошибка: в docker-compose и CI переменные приходят прямо
// из окружения. Уже заданные переменные окружения не перезаписываются, поэтому
// при совпадении ключа выигрывает файл, который ближе к модулю.
func LoadDotEnv() ([]string, error) {
	var loaded []string

	for _, path := range dotEnvCandidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		if err := godotenv.Load(path); err != nil {
			return loaded, fmt.Errorf("не удалось прочитать %s: %w", path, err)
		}

		loaded = append(loaded, path)
	}

	return loaded, nil
}
