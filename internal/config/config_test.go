package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var requiredEnvVars = []string{
	"OBA_API_KEY",
	"HOME_STOP_ID",
	"OFFICE_STOP_ID",
	"HOME_WIFI",
	"OFFICE_WIFI",
	"DEFAULT_LOCATION",
}

var requiredWithoutDefaultLocation = []string{
	"OBA_API_KEY",
	"HOME_WIFI",
	"OFFICE_WIFI",
	"HOME_STOP_ID",
	"OFFICE_STOP_ID",
}

func clearEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range requiredEnvVars {
		t.Setenv(key, "") // save original for cleanup
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
	writeEnvFileContent(t, dir, []byte(content))
}

func writeEnvFileWithBOM(t *testing.T, dir string) {
	t.Helper()
	content := []byte("\ufeffOBA_API_KEY=file-key\nHOME_WIFI=file-home\nOFFICE_WIFI=file-office\nHOME_STOP_ID=file-home-stop\nOFFICE_STOP_ID=file-office-stop\n")
	writeEnvFileContent(t, dir, content)
}

func writeEnvFileContent(t *testing.T, dir string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func setIsolatedConfigDirEnv(t *testing.T) {
	t.Helper()

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
	t.Setenv("APPDATA", filepath.Join(fakeHome, "AppData", "Roaming"))
}

func TestLoadFromEnv_AllPresent(t *testing.T) {
	want := &Config{
		APIKey:          "api-key",
		HomeWifi:        "home-wifi",
		OfficeWifi:      "office-wifi",
		HomeStopID:      "home-stop",
		OfficeStopID:    "office-stop",
		DefaultLocation: "",
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

func TestLoadFromEnv_DefaultLocationMakesWifiOptional(t *testing.T) {
	want := &Config{
		APIKey:          "api-key",
		HomeStopID:      "home-stop",
		OfficeStopID:    "office-stop",
		DefaultLocation: "home",
	}

	t.Setenv("OBA_API_KEY", want.APIKey)
	t.Setenv("HOME_STOP_ID", want.HomeStopID)
	t.Setenv("OFFICE_STOP_ID", want.OfficeStopID)
	t.Setenv("DEFAULT_LOCATION", want.DefaultLocation)
	t.Setenv("HOME_WIFI", "")
	t.Setenv("OFFICE_WIFI", "")

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
			wantContains: requiredWithoutDefaultLocation,
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
			wantContains: requiredWithoutDefaultLocation,
		},
		{
			name: "MissingWiFiWithoutDefaultLocation",
			env: map[string]string{
				"OBA_API_KEY":    "api-key",
				"HOME_STOP_ID":   "home-stop",
				"OFFICE_STOP_ID": "office-stop",
			},
			wantContains: []string{"HOME_WIFI", "OFFICE_WIFI"},
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

func TestLoadFromEnv_InvalidDefaultLocation(t *testing.T) {
	t.Setenv("OBA_API_KEY", "api-key")
	t.Setenv("HOME_STOP_ID", "home-stop")
	t.Setenv("OFFICE_STOP_ID", "office-stop")
	t.Setenv("DEFAULT_LOCATION", "dock")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want non-nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, `invalid DEFAULT_LOCATION "dock"`) {
		t.Fatalf("error = %q, want invalid DEFAULT_LOCATION message", errMsg)
	}
	if strings.Contains(errMsg, "HOME_WIFI") || strings.Contains(errMsg, "OFFICE_WIFI") {
		t.Fatalf("error = %q, should not require wifi vars when DEFAULT_LOCATION is set", errMsg)
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

	// Override config-dir env vars so os.UserConfigDir() resolves to a temp location.
	setIsolatedConfigDirEnv(t)

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

func TestLoad_ConfigDirFallbackWithUTF8BOM(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Chdir(%q): %v", emptyDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	setIsolatedConfigDirEnv(t)

	cfgDir := ConfigDir()
	if cfgDir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	writeEnvFileWithBOM(t, cfgDir)

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
	setIsolatedConfigDirEnv(t)

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

	// Override config-dir env vars so ConfigDir() resolves to a controlled
	// empty location, matching the isolation used by other Load tests.
	setIsolatedConfigDirEnv(t)

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

func TestLoad_InvalidConfigDirEnvFile(t *testing.T) {
	clearEnvVars(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Chdir(%q): %v", emptyDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	setIsolatedConfigDirEnv(t)

	cfgDir := ConfigDir()
	if cfgDir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	writeEnvFileContent(t, cfgDir, []byte("OBA_API_KEY=\"unterminated\n"))

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "parse ") {
		t.Fatalf("error = %q, want to contain %q", errMsg, "parse ")
	}
	if strings.Contains(errMsg, "no .env found") {
		t.Fatalf("error = %q, should not misreport missing file", errMsg)
	}
}

func TestConfigDir(t *testing.T) {
	fakeHome := t.TempDir()
	fakeAppData := filepath.Join(fakeHome, "AppData", "Roaming")

	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
	t.Setenv("APPDATA", fakeAppData)

	dir := ConfigDir()
	if dir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}

	var want string
	switch runtime.GOOS {
	case "linux":
		want = filepath.Join(fakeHome, ".config", appName)
	case "darwin":
		want = filepath.Join(fakeHome, "Library", "Application Support", appName)
	case "windows":
		want = filepath.Join(fakeAppData, appName)
	default:
		if !strings.HasSuffix(dir, appName) {
			t.Fatalf("ConfigDir() = %q, want suffix %q", dir, appName)
		}
		return
	}

	if dir != want {
		t.Fatalf("ConfigDir() = %q, want %q", dir, want)
	}
}
