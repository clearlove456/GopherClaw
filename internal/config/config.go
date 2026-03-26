package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	APIKey       string
	BaseURL      string
	ModelID      string
	SystemPrompt string
	MaxTokens    int
}

func Load() (Config, error) {
	if path, ok := findDotEnv(); ok {
		_ = loadDotEnv(path)
	}

	cfg := Config{
		APIKey:       strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL:      getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		ModelID:      getEnv("MODEL_ID", "gpt-4o-mini"),
		SystemPrompt: getEnv("SYSTEM_PROMPT", "You are a helpful AI assistant. Answer questions directly."),
		MaxTokens:    getEnvInt("MAX_TOKENS", 8096),
	}

	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("Error: OPENAI_API_KEY 未设置.\n将 .env.example 复制为 .env 并填入你的 key.")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func getEnvInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}

	return parsed
}

func findDotEnv() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	cur := wd
	for {
		candidate := filepath.Join(cur, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, true
		}

		next := filepath.Dir(cur)
		if next == cur {
			return "", false
		}
		cur = next
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}

		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}
