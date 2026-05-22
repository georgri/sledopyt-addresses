package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	TelegramToken   string
	KLADRSourcePath string
	GARDataPath     string
	UserStatePath   string
}

func FromEnv() (Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	kladrPath := os.Getenv("KLADR_SOURCE_PATH")
	if kladrPath == "" {
		return Config{}, fmt.Errorf("KLADR_SOURCE_PATH is required")
	}

	statePath := os.Getenv("USER_STATE_PATH")
	if statePath == "" {
		statePath = "data/user_state.json"
	}
	garPath := os.Getenv("GAR_DATA_PATH")
	if garPath == "" {
		garPath = os.Getenv("GAR77_DATA_PATH")
	}

	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return Config{}, fmt.Errorf("create state dir: %w", err)
	}

	return Config{
		TelegramToken:   token,
		KLADRSourcePath: kladrPath,
		GARDataPath:     garPath,
		UserStatePath:   statePath,
	}, nil
}
