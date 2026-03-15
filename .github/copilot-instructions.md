# Copilot Instructions

## Build & Test

```sh
go build -o wheresmybus .
go test ./...
go test -race -coverprofile=coverage.out ./...

# Single test
go test -run TestResolveStop ./...
go test -run TestGetArrivalsFromURL_HTTPError404 ./internal/api/

# Lint (matches CI)
golangci-lint run
```

CI runs lint, tests on all 3 OS (ubuntu, macos, windows), then build. See `.github/workflows/ci.yml`.

## Architecture

CLI tool that shows real-time bus arrivals from the OneBusAway Puget Sound API. Flow:

1. `config.Load()` — reads `.env` via godotenv, validates 5 required env vars
2. `wifi.CurrentSSID()` — detects SSID via platform-specific commands (nmcli/airport/PowerShell/netsh)
3. `resolveStop()` in `main.go` — maps wifi network or `--direction` flag to a stop ID
4. `api.GetArrivals()` — fetches from OBA API, deduplicates by TripID
5. `display.PrintArrivals()` — sorts by time, collapses "bunched" arrivals (same route within 60s), prints table

All internal packages use only the standard library. The only external dependency is `github.com/joho/godotenv`.

## Conventions

**Testing patterns:**
- Table-driven tests with `t.Run()` subtests throughout
- `httptest.NewServer()` for API tests; custom `RoundTripper` for URL rewriting
- `wifi` package uses a swappable `commandRunner` interface (global `runner` var) for mocking OS commands
- `display` tests capture stdout via `os.Pipe()`
- Use `t.Setenv()` for env vars, `t.Cleanup()` for state restoration

**Dependency injection:**
- `resolveStop()` accepts `detectSSID func() (string, error)` — not called when `--direction` is explicit
- `GetArrivalsFromURL()` accepts `*http.Client` for test injection
- `wifi.commandRunner` interface wraps `exec.Command().Output()` calls

**Error handling:**
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Config validation reports all missing vars at once, not one at a time
- WiFi detection degrades gracefully — returns empty string on command failure

**Environment:**
- Configuration lives in `.env` (not committed). See `.env.example` for required vars.
- `OBA_API_KEY`, `HOME_WIFI`, `OFFICE_WIFI`, `HOME_STOP_ID`, `OFFICE_STOP_ID` are all required
