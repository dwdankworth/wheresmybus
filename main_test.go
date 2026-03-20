package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dwdankworth/wheresmybus/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		APIKey:          "test-key",
		HomeWifi:        "HomeNet",
		OfficeWifi:      "OfficeNet",
		HomeStopID:      "1_HOME",
		OfficeStopID:    "1_OFFICE",
		DefaultLocation: "",
	}
}

func TestResolveStop(t *testing.T) {
	tests := []struct {
		name             string
		stop             string
		direction        string
		defaultLocation  string
		detectSSID       func() (string, error)
		wantStop         string
		wantErr          string
		wantDetectCalled bool
	}{
		{
			name: "explicit stop skips wifi detection",
			stop: "12345",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "12345",
			wantDetectCalled: false,
		},
		{
			name: "explicit full stop ID skips wifi detection",
			stop: "1_75403",
			detectSSID: func() (string, error) {
				return "OfficeNet", nil
			},
			wantStop:         "1_75403",
			wantDetectCalled: false,
		},
		{
			name:      "explicit home",
			direction: "home",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: false,
		},
		{
			name:      "explicit office",
			direction: "office",
			detectSSID: func() (string, error) {
				return "OfficeNet", nil
			},
			wantStop:         "1_HOME",
			wantDetectCalled: false,
		},
		{
			name:      "wifi detects home network",
			direction: "",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_HOME",
			wantDetectCalled: true,
		},
		{
			name:      "wifi detects office network",
			direction: "",
			detectSSID: func() (string, error) {
				return "OfficeNet", nil
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: true,
		},
		{
			name:      "unknown wifi network",
			direction: "",
			detectSSID: func() (string, error) {
				return "UnknownNet", nil
			},
			wantErr:          "unknown wifi network",
			wantDetectCalled: true,
		},
		{
			name:            "default location used when wifi is unavailable",
			direction:       "",
			defaultLocation: "home",
			detectSSID: func() (string, error) {
				return "", nil
			},
			wantStop:         "1_HOME",
			wantDetectCalled: true,
		},
		{
			name:            "default location used when wifi detection errors",
			direction:       "",
			defaultLocation: "office",
			detectSSID: func() (string, error) {
				return "", errors.New("boom")
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: true,
		},
		{
			name:            "wifi match wins over default location",
			direction:       "",
			defaultLocation: "office",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_HOME",
			wantDetectCalled: true,
		},
		{
			name:            "explicit direction wins over default location",
			direction:       "home",
			defaultLocation: "office",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: false,
		},
		{
			name:      "no wifi connection",
			direction: "",
			detectSSID: func() (string, error) {
				return "", nil
			},
			wantErr:          "not connected to wifi",
			wantDetectCalled: true,
		},
		{
			name:      "wifi detection error",
			direction: "",
			detectSSID: func() (string, error) {
				return "", errors.New("boom")
			},
			wantErr:          "wifi detection failed",
			wantDetectCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			detectSSID := func() (string, error) {
				called = true
				return tt.detectSSID()
			}

			cfg := testConfig()
			cfg.DefaultLocation = tt.defaultLocation

			got, err := resolveStop(cfg, tt.stop, tt.direction, detectSSID)

			if called != tt.wantDetectCalled {
				t.Fatalf("detectSSID called = %v, want %v", called, tt.wantDetectCalled)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveStop returned unexpected error: %v", err)
			}
			if got != tt.wantStop {
				t.Fatalf("stop = %q, want %q", got, tt.wantStop)
			}
		})
	}
}

func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name       string
		stop       string
		direction  string
		maxResults int
		wantErr    string
	}{
		{
			name:       "stop and direction conflict",
			stop:       "12345",
			direction:  "home",
			maxResults: 10,
			wantErr:    "-stop and -direction cannot be used together",
		},
		{
			name:       "invalid direction",
			direction:  "elsewhere",
			maxResults: 10,
			wantErr:    "-direction must be 'home' or 'office'",
		},
		{
			name:       "stop only is valid",
			stop:       "12345",
			maxResults: 10,
		},
		{
			name:       "direction only is valid",
			direction:  "office",
			maxResults: 10,
		},
		{
			name:       "zero max results is valid",
			maxResults: 0,
		},
		{
			name:       "negative max results is invalid",
			maxResults: -1,
			wantErr:    "-max-results must be 0 or greater",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlags(tt.stop, tt.direction, tt.maxResults)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateFlags returned unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestVersionOutput(t *testing.T) {
	originalVersion := version
	t.Cleanup(func() {
		version = originalVersion
	})

	tests := []struct {
		name        string
		version     string
		wantVersion string
	}{
		{name: "explicit version", version: "v1.2.3", wantVersion: "v1.2.3"},
		{name: "empty version falls back to dev", version: "", wantVersion: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version = tt.version

			got := versionString()
			want := "wheresmybus version " + tt.wantVersion

			if got != want {
				t.Fatalf("version output = %q, want %q", got, want)
			}
		})
	}
}
