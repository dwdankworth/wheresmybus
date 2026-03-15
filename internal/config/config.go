package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const appName = "wheresmybus"

// Config contains runtime settings loaded from environment variables.
type Config struct {
	APIKey       string
	HomeWifi     string
	OfficeWifi   string
	HomeStopID   string
	OfficeStopID string
}

// ConfigDir returns the platform-specific config directory for wheresmybus.
// Returns an empty string if the directory cannot be determined.
func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, appName)
}

// Load reads configuration from a .env file, searching in priority order:
//  1. .env in the current working directory
//  2. .env in the user config directory (e.g. ~/.config/wheresmybus/)
//
// If no .env file is found, it falls back to reading from environment
// variables directly.
func Load() (*Config, error) {
	loaded := false

	// Try CWD first
	if err := godotenv.Load(); err == nil {
		loaded = true
	}

	// Try user config dir
	if !loaded {
		if cfgDir := ConfigDir(); cfgDir != "" {
			cfgPath := filepath.Join(cfgDir, ".env")
			if err := godotenv.Load(cfgPath); err == nil {
				loaded = true
			}
		}
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		if !loaded {
			return nil, fmt.Errorf("%w\n\nNo .env file found. Searched:\n  1. .env (current directory)\n  2. %s\n\nRun the setup script or copy .env.example to one of the above locations.",
				err, configFilePath())
		}
		return nil, err
	}
	return cfg, nil
}

func configFilePath() string {
	if dir := ConfigDir(); dir != "" {
		return filepath.Join(dir, ".env")
	}
	return "<config dir>/.env (could not determine config directory)"
}

// LoadFromEnv reads configuration from environment variables.
func LoadFromEnv() (*Config, error) {
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
