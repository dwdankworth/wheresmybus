# TUI Migration Plan

## Problem

The project is currently a one-shot CLI: it resolves a stop from flags or Wi-Fi-backed home/office config, fetches arrivals, and prints a table. The new goal is an interactive TUI that lets users search for stops, save favorite stops, and reopen those saved stops across sessions without losing the existing utility commands that are still useful outside the TUI.

## Current State

- `main.go` owns flag parsing, config loading, stop resolution, API calls, and final rendering.
- `internal/config` only loads `.env` / environment variables and currently requires `HOME_STOP_ID` and `OFFICE_STOP_ID` plus Wi-Fi/default-location settings.
- `internal/api` supports arrivals only; there is no stop discovery or persistence layer.
- `internal/display` renders a static ASCII table to stdout, which is fine for the CLI path but not reusable as a stateful interface.
- `setup.sh`, `setup.ps1`, `.env.example`, and `README.md` all assume an env-driven home/office workflow.
- CI currently validates with lint, tests, race+coverage tests, and build on Linux/macOS/Windows.

## Proposed Approach

1. Keep the non-interactive utility paths (`-version`, `-update`, `--print-config-dir`) as fast exits, and keep `-stop` plus `-max-results` as the simple non-interactive direct lookup path for automation.
2. Move the interactive default path into a new `internal/tui` package, ideally using Bubble Tea/Bubbles so a smaller implementation agent can work with a standard Go update/view model instead of hand-rolled terminal control.
3. Simplify configuration:
   - keep `.env` for secrets, with `OBA_API_KEY` as the only always-required variable
   - remove the home/office + Wi-Fi model from the primary product flow
   - store saved stops in a separate JSON file under the existing platform config directory, because user-managed favorites are a poor fit for `.env`
4. Expand `internal/api` with stop discovery, using the OneBusAway Search API endpoint `/api/where/search/stop.json?input=...` for autocomplete-style stop lookup and, if needed, `/api/where/stop/{id}.json` to refresh saved-stop metadata.
5. Launch the TUI by default, keep `-stop` plus `-max-results` as the non-interactive shortcut, and retire the `-direction` / Wi-Fi-driven stop resolution path.

## Implementation Todos

### 1. Reshape runtime config and persistence

- Make `OBA_API_KEY` the only always-required env var for the TUI path.
- Remove `HOME_WIFI`, `OFFICE_WIFI`, `HOME_STOP_ID`, `OFFICE_STOP_ID`, and `DEFAULT_LOCATION` from the primary startup contract.
- Add a persisted saved-stop store in the config directory, for example `saved_stops.json`.
- Define a saved-stop model with stable fields such as label, stop ID, display name, route hints, and last-updated metadata.
- Add tests for config validation, first-run empty state, malformed JSON, save/load round-trips, and minimal-config startup.

**Likely files:** `internal/config/config.go`, `internal/config/config_test.go`, new persistence file(s) under `internal/config/` or a new `internal/storage/` package.

### 2. Add stop search and normalization APIs

- Add an API method such as `SearchStops(apiKey, input string, maxCount int)` backed by `/api/where/search/stop.json`.
- Parse `data.list` stop entries directly; this endpoint already returns stop-only results, so no mixed-type filtering step is needed.
- Persist the exact returned stop `id` as the canonical OBA stop ID (for example `1_75403` or `19_855`); do not synthesize or re-prefix saved search results with `1_`.
- Derive optional route hints from `references.routes` by matching each stop's `routeIds` to route `shortName` / `nullSafeShortName`, but do not block v1 favorites on perfect route-label enrichment.
- Add `GetStopByID` via `/api/where/stop/{id}.json` only if the TUI needs to hydrate saved-stop metadata independently of arrival fetches.
- Add tests for successful stop search, empty results, malformed JSON, canonical multi-agency IDs, optional stop lookup, and HTTP / OBA error handling.

**Likely files:** `internal/api/client.go`, `internal/api/client_test.go`.

### 3. Build the TUI shell

- Create a new `internal/tui` package with a root model and focused submodels/helpers for:
  - saved stops list
  - search input + results
  - arrivals detail view
  - transient loading / error states
- Support a minimal first-pass keymap:
  - arrow keys or `j`/`k` to move
  - `enter` to open arrivals for the selected stop
  - `/` or `s` to start a search
  - `a` to save the selected search result
  - `d` to delete a saved stop
  - `r` to refresh arrivals
  - `q` to quit
- Keep the first implementation intentionally simple: one active pane at a time is easier for a smaller agent than a multi-pane dashboard.
- Reuse existing arrival formatting logic where practical, but be willing to extract shared presentation helpers instead of forcing the old `display.PrintArrivals` API into the TUI.

**Likely files:** new `internal/tui/*.go`, possible refactor in `internal/display/`.

### 4. Rewire the entrypoint around the TUI

- Keep early returns for `-version`, `-update`, and `--print-config-dir`.
- Keep `-stop` plus `-max-results` as the direct non-interactive path, and remove `-direction` plus the old `resolveStop` Wi-Fi/home-office branch from the main experience.
- Construct the dependencies once in `main.go` and inject them into the TUI model rather than letting the TUI call global functions directly.
- Preserve current updater/config-dir behavior unchanged.
- Delete `internal/wifi` and related tests if nothing else uses them after the migration.
- Update main tests to cover the new startup branching.

**Likely files:** `main.go`, `main_test.go`.

### 5. Update setup and docs

- Update `.env.example` so it reflects the new minimum setup, centered on the API key and saved-stop workflow.
- Update `setup.sh` and `setup.ps1` so first-run setup matches the new config rules and no longer interviews for home/office Wi-Fi metadata.
- Document the TUI workflow, keybindings, search/save behavior, and the remaining `-stop` / `-max-results` compatibility path in `README.md`.
- Keep this file current so another implementation agent can work directly from it.

**Likely files:** `.env.example`, `setup.sh`, `setup.ps1`, `README.md`.

### 6. Validate the migration

- `golangci-lint run`
- `go test ./...`
- `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
- `go build -o wheresmybus .`

## Important Decisions and Assumptions

- **Recommended TUI stack:** Bubble Tea + Bubbles, because that is the simplest path for a smaller coding agent to implement correctly and test incrementally.
- **Persistence choice:** use a JSON state file in the platform config directory instead of trying to encode a mutable favorites list inside `.env`.
- **Primary product decision:** replace the home/office model with saved stops as the main interaction model.
- **Validated OBA surface:** arrivals remain `/api/where/arrivals-and-departures-for-stop/{id}.json`, stop autocomplete is `/api/where/search/stop.json` with `input` and `maxCount`, and optional stop hydration is `/api/where/stop/{id}.json`.
- **ID handling rule:** save the exact stop `id` returned by OneBusAway search or stop lookup; only the manual direct-entry path should infer `1_<code>` from a bare numeric stop code.
- **Compatibility stance:** keep updater/version/config-dir behavior intact, keep `-stop` plus `-max-results` as the non-interactive escape hatch, and retire `-direction` plus Wi-Fi-driven stop resolution.
- **Search scope assumption:** the initial TUI should support text-based stop discovery through OneBusAway search; location-aware ranking can wait unless explicitly requested.
