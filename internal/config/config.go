package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	BotToken       string
	GeminiAPIKey   string
	GeminiModel    string
	DatabasePath   string
	Timezone       string
	AllowedUserIDs map[int64]struct{}
}

func Load() (*Config, error) {
	botToken := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if botToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}

	geminiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if geminiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = "gemini-3.1-flash-lite"
	}

	dbPath := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if dbPath == "" {
		dbPath = "data/kbju.db"
	}

	tz := strings.TrimSpace(os.Getenv("TIMEZONE"))
	if tz == "" {
		tz = "Europe/Moscow"
	}

	allowed, err := parseAllowedUserIDs(os.Getenv("ALLOWED_USER_IDS"))
	if err != nil {
		return nil, err
	}

	return &Config{
		BotToken:       botToken,
		GeminiAPIKey:   geminiKey,
		GeminiModel:    model,
		DatabasePath:   dbPath,
		Timezone:       tz,
		AllowedUserIDs: allowed,
	}, nil
}

func parseAllowedUserIDs(raw string) (map[int64]struct{}, error) {
	raw = strings.TrimSpace(raw)
	out := make(map[int64]struct{})
	if raw == "" {
		return out, nil
	}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ALLOWED_USER_IDS entry %q: %w", part, err)
		}
		out[id] = struct{}{}
	}
	return out, nil
}

func (c *Config) IsAllowed(userID int64) bool {
	if len(c.AllowedUserIDs) == 0 {
		return true
	}
	_, ok := c.AllowedUserIDs[userID]
	return ok
}
