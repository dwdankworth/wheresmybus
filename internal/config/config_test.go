package config

import (
	"os"
	"path/filepath"
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

func clearEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range requiredEnvVars {
		t.Setenv(key, "")  // save original for cleanup
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%q): %v", key, err)
		}
	}
}

func writeEnvFile(t *testing.T, dir string) {
	t.Helper()
	content := `OBA_API_KEY=file-key
HOME_WIFI=file-home
OFFICE_WIFI=file-office
HOME_STOP_ID=file-home-stop
OFFICE_STOP_ID=file-office-stop
`
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
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

func TestLoad_CWDEnvFile(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	tempDir := t.TempDir()
	writeEnvFile(t, tempDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%q): %v", tempDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "file-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "file-key")
	}
}

func TestLoad_ConfigDirFallback(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	// CWD with no .env
	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Chdir(%q): %v", emptyDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Override HOME/APPDATA so os.UserConfigDir() resolves to a temp location
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("APPDATA", filepath.Join(fakeHome, "AppData", "Roaming"))

	cfgDir := ConfigDir()
	if cfgDir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	writeEnvFile(t, cfgDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "file-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "file-key")
	}
}

func TestLoad_CWDTakesPriority(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	// CWD .env with distinct values
	cwdDir := t.TempDir()
	cwdContent := `OBA_API_KEY=cwd-key
HOME_WIFI=cwd-home
OFFICE_WIFI=cwd-office
HOME_STOP_ID=cwd-home-stop
OFFICE_STOP_ID=cwd-office-stop
`
	if err := os.WriteFile(filepath.Join(cwdDir, ".env"), []byte(cwdContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Config dir .env with different values
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("APPDATA", filepath.Join(fakeHome, "AppData", "Roaming"))

	cfgDir := ConfigDir()
	if cfgDir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	writeEnvFile(t, cfgDir)

	if err := os.Chdir(cwdDir); err != nil {
		t.Fatalf("Chdir(%q): %v", cwdDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "cwd-key" {
		t.Fatalf("APIKey = %q, want %q (CWD should take priority)", cfg.APIKey, "cwd-key")
	}
}

func TestLoad_EnvVarsWithoutFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Chdir(%q): %v", emptyDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Set env vars directly — should work without any .env file
	t.Setenv("OBA_API_KEY", "env-key")
	t.Setenv("HOME_WIFI", "env-home")
	t.Setenv("OFFICE_WIFI", "env-office")
	t.Setenv("HOME_STOP_ID", "env-home-stop")
	t.Setenv("OFFICE_STOP_ID", "env-office-stop")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
}

func TestLoad_NoEnvFile(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Use an empty temp dir as CWD (no .env here)
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tempDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Override HOME/APPDATA so ConfigDir() resolves to a controlled
	// empty location, matching the isolation used by other Load tests.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("APPDATA", filepath.Join(fakeHome, "AppData", "Roaming"))

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "missing required environment variables") {
		t.Fatalf("error = %q, want to contain %q", errMsg, "missing required environment variables")
	}
	if !strings.Contains(errMsg, "no .env found") {
		t.Fatalf("error = %q, want to contain %q", errMsg, "no .env found")
	}
}

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	if !strings.HasSuffix(dir, appName) {
		t.Fatalf("ConfigDir() = %q, want suffix %q", dir, appName)
	}
}
