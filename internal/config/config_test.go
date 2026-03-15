package config

import (
	"os"
	"strings"
	"testing"
)

var requiredEnvVars = []string{
	"OBA_API_KEY",
	"HOME_WIFI",
	"OFFICE_WIFI",
	"HOME_STOP_ID",
	"OFFICE_STOP_ID",
}

func TestLoadFromEnv_AllPresent(t *testing.T) {
	want := &Config{
		APIKey:       "api-key",
		HomeWifi:     "home-wifi",
		OfficeWifi:   "office-wifi",
		HomeStopID:   "home-stop",
		OfficeStopID: "office-stop",
	}

	t.Setenv("OBA_API_KEY", want.APIKey)
	t.Setenv("HOME_WIFI", want.HomeWifi)
	t.Setenv("OFFICE_WIFI", want.OfficeWifi)
	t.Setenv("HOME_STOP_ID", want.HomeStopID)
	t.Setenv("OFFICE_STOP_ID", want.OfficeStopID)

	got, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if *got != *want {
		t.Fatalf("LoadFromEnv() = %+v, want %+v", *got, *want)
	}
}

func TestLoadFromEnv_MissingCases(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "MissingAll",
			wantContains: requiredEnvVars,
		},
		{
			name: "MissingSome",
			env: map[string]string{
				"OBA_API_KEY": "api-key",
				"HOME_WIFI":   "home-wifi",
			},
			wantContains:   []string{"OFFICE_WIFI", "HOME_STOP_ID", "OFFICE_STOP_ID"},
			wantNotContain: []string{"OBA_API_KEY", "HOME_WIFI"},
		},
		{
			name: "EmptyValues",
			env: map[string]string{
				"OBA_API_KEY":    "",
				"HOME_WIFI":      "",
				"OFFICE_WIFI":    "",
				"HOME_STOP_ID":   "",
				"OFFICE_STOP_ID": "",
			},
			wantContains: requiredEnvVars,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range requiredEnvVars {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := LoadFromEnv()
			if err == nil {
				t.Fatal("LoadFromEnv() error = nil, want non-nil")
			}

			errMsg := err.Error()
			for _, want := range tt.wantContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("error %q does not contain %q", errMsg, want)
				}
			}
			for _, unwanted := range tt.wantNotContain {
				if strings.Contains(errMsg, unwanted) {
					t.Errorf("error %q unexpectedly contains %q", errMsg, unwanted)
				}
			}
		})
	}
}

func TestLoad_NoEnvFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tempDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore working directory to %q: %v", cwd, err)
		}
	})

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "load .env") {
		t.Fatalf("Load() error = %q, want to contain %q", err.Error(), "load .env")
	}

	if !strings.Contains(err.Error(), ".env") {
		t.Fatalf("Load() error = %q, want to mention .env", err.Error())
	}
}
