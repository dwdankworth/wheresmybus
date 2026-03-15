package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains runtime settings loaded from environment variables.
type Config struct {
	APIKey       string
	HomeWifi     string
	OfficeWifi   string
	HomeStopID   string
	OfficeStopID string
}

// Load reads configuration from a .env file in the current directory.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg := &Config{
		APIKey:       os.Getenv("OBA_API_KEY"),
		HomeWifi:     os.Getenv("HOME_WIFI"),
		OfficeWifi:   os.Getenv("OFFICE_WIFI"),
		HomeStopID:   os.Getenv("HOME_STOP_ID"),
		OfficeStopID: os.Getenv("OFFICE_STOP_ID"),
	}

	missing := make([]string, 0, 5)
	if cfg.APIKey == "" {
		missing = append(missing, "OBA_API_KEY")
	}
	if cfg.HomeWifi == "" {
		missing = append(missing, "HOME_WIFI")
	}
	if cfg.OfficeWifi == "" {
		missing = append(missing, "OFFICE_WIFI")
	}
	if cfg.HomeStopID == "" {
		missing = append(missing, "HOME_STOP_ID")
	}
	if cfg.OfficeStopID == "" {
		missing = append(missing, "OFFICE_STOP_ID")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
