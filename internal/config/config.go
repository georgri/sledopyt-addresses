package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	TelegramToken      string
	KLADRSourcePath    string
	GARDataPath        string
	UserStatePath      string
	GeocodeCachePath   string
	NominatimUserAgent string
	NominatimEmail     string
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
	geoCachePath := os.Getenv("GEOCODE_CACHE_PATH")
	if geoCachePath == "" {
		geoCachePath = "data/geocode_cache.json"
	}
	nominatimUserAgent := os.Getenv("NOMINATIM_USER_AGENT")
	nominatimEmail := os.Getenv("NOMINATIM_EMAIL")

	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return Config{}, fmt.Errorf("create state dir: %w", err)
	}

	return Config{
		TelegramToken:      token,
		KLADRSourcePath:    kladrPath,
		GARDataPath:        garPath,
		UserStatePath:      statePath,
		GeocodeCachePath:   geoCachePath,
		NominatimUserAgent: nominatimUserAgent,
		NominatimEmail:     nominatimEmail,
	}, nil
}
