package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dwdan/wheresmybus/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		APIKey:       "test-key",
		HomeWifi:     "HomeNet",
		OfficeWifi:   "OfficeNet",
		HomeStopID:   "1_HOME",
		OfficeStopID: "1_OFFICE",
	}
}

func TestResolveStop(t *testing.T) {
	tests := []struct {
		name             string
		direction        string
		detectSSID       func() (string, error)
		wantStop         string
		wantErr          string
		wantDetectCalled bool
	}{
		{
			name:      "explicit home",
			direction: "home",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_HOME",
			wantDetectCalled: false,
		},
		{
			name:      "explicit office",
			direction: "office",
			detectSSID: func() (string, error) {
				return "OfficeNet", nil
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: false,
		},
		{
			name:      "wifi detects home network",
			direction: "",
			detectSSID: func() (string, error) {
				return "HomeNet", nil
			},
			wantStop:         "1_OFFICE",
			wantDetectCalled: true,
		},
		{
			name:      "wifi detects office network",
			direction: "",
			detectSSID: func() (string, error) {
				return "OfficeNet", nil
			},
			wantStop:         "1_HOME",
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

			got, err := resolveStop(testConfig(), tt.direction, detectSSID)

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
