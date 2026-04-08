package main

import (
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
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

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if testVersion := os.Getenv("GO_WANT_TEST_VERSION"); testVersion != "" {
		version = testVersion
	}

	if target := os.Getenv("GO_WANT_REWRITE_URL"); target != "" {
		targetURL, err := url.Parse(target)
		if err != nil {
			panic(err)
		}

		http.DefaultClient = &http.Client{
			Transport: rewriteTransport{base: http.DefaultTransport, target: targetURL},
		}
	}

	args := []string{"wheresmybus"}
	for i, arg := range os.Args {
		if arg == "--" {
			args = append(args, os.Args[i+1:]...)
			break
		}
	}

	os.Args = args
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
	os.Exit(0)
}

func runMain(t *testing.T, env map[string]string, args ...string) (string, int) {
	t.Helper()

	cmdArgs := append([]string{"-test.run=TestMainHelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run main subprocess: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}

	return string(output), exitCode
}

// Verifies that -version prints the version string and exits before config loading.
// Mutation detected: remove the early return in the -version branch so main tries to load config and exits with an error.
func TestMain_PrintsVersionWithoutLoadingConfig(t *testing.T) {
	output, exitCode := runMain(t, map[string]string{
		"GO_WANT_TEST_VERSION": "v9.9.9",
	}, "-version")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 with output %q", exitCode, output)
	}
	if strings.TrimSpace(output) != "wheresmybus version v9.9.9" {
		t.Fatalf("output = %q, want version string", output)
	}
}

// Verifies that -print-config-dir prints the platform config directory and exits before config loading.
// Mutation detected: delete the -print-config-dir branch so the CLI falls through to config loading instead of printing the platform config directory.
func TestMain_PrintsConfigDirWithoutLoadingConfig(t *testing.T) {
	configHome := t.TempDir()

	// Set the same env vars we pass to the subprocess so config.ConfigDir()
	// returns the platform-correct expected path (e.g. on macOS it uses
	// $HOME/Library/Application Support, not XDG_CONFIG_HOME).
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)

	want := config.ConfigDir()
	if want == "" {
		t.Fatal("config.ConfigDir() returned empty string")
	}

	output, exitCode := runMain(t, map[string]string{
		"HOME":            configHome,
		"XDG_CONFIG_HOME": configHome,
		"APPDATA":         configHome,
	}, "-print-config-dir")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 with output %q", exitCode, output)
	}
	if strings.TrimSpace(output) != want {
		t.Fatalf("output = %q, want config dir %q", output, want)
	}
}

// Verifies that configuration load failures print setup guidance and exit non-zero.
// Mutation detected: remove the config.Load error handling block so startup failures lose the setup guidance or exit successfully.
func TestMain_LoadFailurePrintsSetupGuidance(t *testing.T) {
	emptyHome := t.TempDir()
	workingDir := t.TempDir()

	output, exitCode := runMain(t, map[string]string{
		"HOME":             emptyHome,
		"XDG_CONFIG_HOME":  emptyHome,
		"APPDATA":          emptyHome,
		"PWD":              workingDir,
		"OBA_API_KEY":      "",
		"HOME_WIFI":        "",
		"OFFICE_WIFI":      "",
		"HOME_STOP_ID":     "",
		"OFFICE_STOP_ID":   "",
		"DEFAULT_LOCATION": "",
	}, "-stop", "75403")

	if exitCode == 0 {
		t.Fatalf("exit code = %d, want non-zero with output %q", exitCode, output)
	}
	if !strings.Contains(output, "Run the setup script or copy .env.example to the config directory.") {
		t.Fatalf("output = %q, want setup guidance", output)
	}
}

// Verifies that an explicit stop fetches arrivals and prints the table without wifi detection.
// Mutation detected: delete the explicit-stop fast path or the display call so the CLI either tries wifi detection or fails to render the fetched arrival.
func TestMain_WithExplicitStopPrintsArrivalsTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/where/arrivals-and-departures-for-stop/1_75403.json" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[{"routeShortName":"8","tripHeadsign":"Seattle Center","predictedArrivalTime":4102445100000,"scheduledArrivalTime":4102445160000,"numberOfStopsAway":2,"predicted":true,"routeId":"1_100236","stopId":"1_75403","tripId":"1_567890"}]}}}`))
	}))
	defer server.Close()

	output, exitCode := runMain(t, map[string]string{
		"GO_WANT_REWRITE_URL": server.URL,
		"OBA_API_KEY":         "integration-key",
		"HOME_WIFI":           "Wallingford-Loft",
		"OFFICE_WIFI":         "Fremont-Hub",
		"HOME_STOP_ID":        "1_75403",
		"OFFICE_STOP_ID":      "1_75548",
	}, "-stop", "75403", "-max-results", "1")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 with output %q", exitCode, output)
	}
	if !strings.Contains(output, "Arrivals for stop 1_75403:") {
		t.Fatalf("output = %q, want resolved stop header", output)
	}
	if !strings.Contains(output, "Seattle Center") || !strings.Contains(output, "2 stops away") {
		t.Fatalf("output = %q, want fetched arrival row", output)
	}
}
