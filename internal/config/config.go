package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	BotToken      string
	OwnerID       int64
	DBPath        string
	WebhookSecret string
	PublicBaseURL string // e.g. https://<app>.up.railway.app, empty => long polling
	Port          string
	WaterRateRiel float64
	ElecRateRiel  float64
	USDRateRiel   float64
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		DBPath:        getEnvDefault("DB_PATH", "data/ptas.db"),
		WebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
		Port:          getEnvDefault("PORT", "8080"),
	}

	if cfg.BotToken == "" {
		return cfg, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	ownerIDStr := os.Getenv("OWNER_TELEGRAM_ID")
	if ownerIDStr == "" {
		return cfg, fmt.Errorf("OWNER_TELEGRAM_ID is required")
	}
	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil {
		return cfg, fmt.Errorf("OWNER_TELEGRAM_ID must be an integer: %w", err)
	}
	cfg.OwnerID = ownerID

	cfg.WaterRateRiel, err = getEnvFloatDefault("WATER_RATE_RIEL", 2500)
	if err != nil {
		return cfg, err
	}
	cfg.ElecRateRiel, err = getEnvFloatDefault("ELEC_RATE_RIEL", 1500)
	if err != nil {
		return cfg, err
	}
	cfg.USDRateRiel, err = getEnvFloatDefault("USD_RATE_RIEL", 4000)
	if err != nil {
		return cfg, err
	}

	if cfg.PublicBaseURL != "" && cfg.WebhookSecret == "" {
		return cfg, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required when PUBLIC_BASE_URL is set")
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvFloatDefault(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return f, nil
}
