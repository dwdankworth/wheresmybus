package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const appName = "wheresmybus"

// Config contains runtime settings loaded from environment variables.
type Config struct {
	APIKey          string
	HomeWifi        string
	OfficeWifi      string
	HomeStopID      string
	OfficeStopID    string
	DefaultLocation string
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
	loaded, err := loadFirstEnvFile()
	if err != nil {
		return nil, err
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		if !loaded {
			return nil, fmt.Errorf("%w (no .env found in current directory or %s)",
				err, configFilePath())
		}
		return nil, err
	}
	return cfg, nil
}

func loadFirstEnvFile() (bool, error) {
	if loaded, err := loadEnvFile(".env"); loaded || err != nil {
		return loaded, err
	}

	if cfgDir := ConfigDir(); cfgDir != "" {
		return loadEnvFile(filepath.Join(cfgDir, ".env"))
	}

	return false, nil
}

func loadEnvFile(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	envVars, err := godotenv.Unmarshal(strings.TrimPrefix(string(content), "\ufeff"))
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}

	for key, value := range envVars {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return false, fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}

	return true, nil
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
		APIKey:          os.Getenv("OBA_API_KEY"),
		HomeWifi:        os.Getenv("HOME_WIFI"),
		OfficeWifi:      os.Getenv("OFFICE_WIFI"),
		HomeStopID:      os.Getenv("HOME_STOP_ID"),
		OfficeStopID:    os.Getenv("OFFICE_STOP_ID"),
		DefaultLocation: os.Getenv("DEFAULT_LOCATION"),
	}

	missing := make([]string, 0, 5)
	if cfg.APIKey == "" {
		missing = append(missing, "OBA_API_KEY")
	}
	if cfg.HomeStopID == "" {
		missing = append(missing, "HOME_STOP_ID")
	}
	if cfg.OfficeStopID == "" {
		missing = append(missing, "OFFICE_STOP_ID")
	}
	if cfg.DefaultLocation == "" {
		if cfg.HomeWifi == "" {
			missing = append(missing, "HOME_WIFI")
		}
		if cfg.OfficeWifi == "" {
			missing = append(missing, "OFFICE_WIFI")
		}
	}

	problems := make([]string, 0, 2)
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("missing required environment variables: %s", strings.Join(missing, ", ")))
	}
	if cfg.DefaultLocation != "" && cfg.DefaultLocation != "home" && cfg.DefaultLocation != "office" {
		problems = append(problems, fmt.Sprintf("invalid DEFAULT_LOCATION %q: must be 'home' or 'office'", cfg.DefaultLocation))
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}

	return cfg, nil
}
