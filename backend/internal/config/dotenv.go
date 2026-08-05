package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var dotEnvCandidates = []string{
	".env",
	filepath.Join("..", ".env"),
}

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
