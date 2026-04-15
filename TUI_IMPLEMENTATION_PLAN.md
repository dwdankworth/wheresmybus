# TUI Migration Implementation Plan

## Objective

Replace the default no-argument CLI flow with an interactive TUI that lets users search for stops, save favorites, reopen those favorites across sessions, and inspect arrivals without losing the existing utility commands that are still valuable outside the TUI.

## Target Behavior

| Invocation | Behavior | Required runtime inputs |
| --- | --- | --- |
| `wheresmybus -version` | Print version and exit immediately | none |
| `wheresmybus --print-config-dir` | Print platform config dir and exit immediately | none |
| `wheresmybus -update` | Check/apply release update and exit immediately | none |
| `wheresmybus -stop <stop> [-max-results N]` | Run the existing non-interactive lookup path and print the ASCII arrivals table | `OBA_API_KEY` only |
| `wheresmybus` | Launch the TUI, load saved stops, and use search/save/delete/refresh flows | `OBA_API_KEY` only |
| `wheresmybus -direction <...>` | Return an explicit deprecation error telling the user to launch the TUI or use `-stop` | none |
| `wheresmybus -max-results <N>` without `-stop` | Return a validation error instead of silently ignoring the flag in TUI mode | none |

## Explicit Scope

### In scope

- Default TUI launch path
- Search-backed stop discovery
- Persistent saved stops in the config directory
- Reuse of existing arrivals logic where possible
- Retained non-interactive `-stop` + `-max-results` path
- Retained `-version`, `-update`, and `--print-config-dir` fast exits
- Removal of Wi-Fi/home-office stop resolution from the main product flow
- Cross-platform behavior on Linux, macOS, and Windows

### Out of scope for v1

- Automatic migration of `HOME_STOP_ID` / `OFFICE_STOP_ID` into saved favorites
- Live per-keystroke network search; explicit submit is acceptable for the first pass
- Multi-pane dashboards or background polling loops
- Location-aware ranking or geolocation features
- In-TUI stop renaming/editing beyond saving a reasonable display label

## Current Surface Area To Account For

- `main.go` and `main_test.go` currently own startup branching, config loading, Wi-Fi-backed stop resolution, direct stop lookup, updater fast exits, and most subprocess-level behavior checks.
- `internal/config/config.go` and `internal/config/config_test.go` currently define the env contract, `.env` precedence rules, config-dir lookup, and user-facing config validation failures.
- `internal/api/client.go` and `internal/api/client_test.go` currently support arrivals only, including bare numeric stop-code normalization and error handling.
- `internal/display/display.go` and `internal/display/display_test.go` currently own arrival sorting, bunch-collapse, ETA/status formatting, truncation, and ASCII table rendering.
- `internal/wifi/wifi.go` and `internal/wifi/wifi_test.go` currently back the home/office auto-resolution path that the migration will retire.
- `setup.sh`, `setup.ps1`, `.env.example`, and `README.md` currently assume an env-driven home/office + Wi-Fi workflow.
- `.github/workflows/ci.yml` and `.github/workflows/release.yml` validate/build/package the repo and ship `README.md` plus `.env.example` in release archives.
- `go.mod` currently has only `github.com/joho/godotenv`; a Bubble Tea implementation will expand the dependency surface and must update `go.sum`.

## Locked Decisions

- **TUI stack:** use Bubble Tea + Bubbles for the first implementation.
- **Primary interaction model:** saved stops replace home/office + Wi-Fi as the default product flow.
- **Config contract:** `OBA_API_KEY` becomes the only required env var for both the TUI path and the retained `-stop` path.
- **Legacy env vars:** `HOME_WIFI`, `OFFICE_WIFI`, `HOME_STOP_ID`, `OFFICE_STOP_ID`, and `DEFAULT_LOCATION` may still exist in old `.env` files but are no longer required and are ignored by the new default workflow.
- **Persistence location:** store saved stops in a JSON file inside the existing platform config directory, separate from `.env`.
- **Persistence filename:** use `saved_stops.json`.
- **Persistence schema:** include a top-level version so future migrations are possible without guessing.
- **Corrupt persistence behavior:** a malformed saved-stops file must never panic; it should surface an explicit, recoverable TUI error and must not be silently overwritten.
- **ID handling:** persist the exact stop `id` returned by OneBusAway search or stop lookup; only manual direct entry should infer `1_<code>` from a bare numeric stop code.
- **Direct CLI compatibility:** `-stop` remains the automation path and must bypass TUI startup and saved-stop loading.
- **Deprecated behavior:** `-direction` stays deprecated with a custom error rather than disappearing as an unknown flag.
- **Search scope:** initial search is text-based stop discovery via OneBusAway search; location-aware ranking can wait.

## Persistence Contract

Use a versioned JSON document at `<config dir>/saved_stops.json`.

Example target shape:

```json
{
  "version": 1,
  "savedStops": [
    {
      "id": "1_75403",
      "label": "Fremont Ave N & N 35th St",
      "name": "Fremont Ave N & N 35th St",
      "code": "75403",
      "direction": "SB",
      "routeHints": ["40", "62"],
      "lastUpdated": "2026-04-15T00:00:00Z"
    }
  ]
}
```

Contract details:

- `id` is the canonical OneBusAway stop ID and is the persistence key.
- `label` is the user-facing saved-stop label; v1 can seed it from the stop name and does not need rename UI.
- `routeHints` are best-effort display hints and do not block saves if missing.
- `lastUpdated` is metadata for future refresh/debuggability, not a runtime requirement.
- Missing file means "first-run empty state", not an error.
- Malformed JSON or unsupported schema version should produce a recoverable TUI error state that points at the file path.
- The app must not silently truncate, discard, or overwrite a corrupt file during startup.

## Sequenced Implementation Plan

### Phase 1 - Relax runtime config and add saved-stop persistence

**Goal:** make API-key-only startup possible and create the persistence layer the TUI will depend on.

**Repo surfaces:**

- `internal/config/config.go`
- `internal/config/config_test.go`
- new `internal/storage/*.go` and tests, or equivalent persistence files if a different package boundary proves cleaner
- `main.go` / `main_test.go` for user-facing config error text

**Required changes:**

1. Keep `ConfigDir()` as the shared source of truth for config file locations.
2. Shrink runtime config loading so `OBA_API_KEY` is the only required environment variable.
3. Preserve current `.env` precedence rules:
   - current working directory `.env`
   - config-directory `.env`
   - already-exported environment variables take precedence over `.env`
4. Stop validating `HOME_WIFI`, `OFFICE_WIFI`, `HOME_STOP_ID`, `OFFICE_STOP_ID`, and `DEFAULT_LOCATION` as required inputs.
5. Add a saved-stop store abstraction with explicit load/save/delete/list behavior.
6. Make the store responsible for:
   - missing file -> empty list
   - valid file -> round-trip load/save
   - malformed file -> explicit error result without auto-overwrite
7. Ensure the retained `-stop` path does not need to load or validate the saved-stop file.
8. Update the startup guidance text in `main.go` so setup instructions match the new minimal config contract.

**Acceptance criteria:**

- A `.env` containing only `OBA_API_KEY` is enough to launch the default TUI path.
- A `.env` containing only `OBA_API_KEY` is enough to run `wheresmybus -stop 75403`.
- Old env vars may be present without affecting behavior.
- Missing `saved_stops.json` produces a clean empty state.
- Malformed `saved_stops.json` produces an explicit, non-panicking error result.

**Tests to add or update:**

- `internal/config/config_test.go`
  - minimal-config success
  - legacy env vars ignored/not required
  - `.env` precedence remains unchanged
  - missing API key remains a hard failure
- new persistence tests
  - first-run empty state
  - save/load round trip
  - delete persistence
  - malformed JSON
  - schema-version handling if introduced immediately
- `main_test.go`
  - updated setup guidance text for config-load failures

### Phase 2 - Expand the OneBusAway client for stop discovery

**Goal:** add the APIs the TUI needs for search and optional stop metadata refresh.

**Repo surfaces:**

- `internal/api/client.go`
- `internal/api/client_test.go`

**Required changes:**

1. Add a stop-search type that carries the data the TUI and saved-stop store need:
   - canonical stop ID
   - display name
   - rider-facing code if available
   - direction or direction text if available
   - best-effort route hints
2. Add `SearchStops(apiKey, input string, maxCount int)` backed by:
   - `/api/where/search/stop.json`
   - `input`
   - `maxCount`
   - `key`
3. Parse `data.list` directly; this endpoint already returns stop-only results.
4. Derive route hints from `references.routes` by matching stop `routeIds` to route `shortName` or `nullSafeShortName`.
5. Preserve exact returned stop IDs for search results and saved stops, including non-`1_` agency prefixes.
6. Keep `GetArrivalsForStop` behavior unchanged for direct CLI use, including bare numeric stop-code normalization.
7. Add `GetStopByID(apiKey, stopID string)` only if the TUI needs to refresh persisted metadata independently of arrivals.
8. Follow the current testability pattern: exported helpers can use `http.DefaultClient`, while internal helpers should accept `*http.Client` and a base URL for tests.

**Acceptance criteria:**

- Search returns canonical IDs without re-prefixing them.
- Empty search results are valid, not errors.
- HTTP failures and OneBusAway failures are surfaced with explicit wrapped errors.
- Direct `-stop` lookup continues to infer `1_<code>` only for bare numeric manual input.

**Tests to add or update:**

- successful stop search
- empty result set
- malformed JSON
- HTTP error handling
- OneBusAway error-code handling
- multi-agency canonical IDs
- route-hint enrichment best-effort behavior
- optional stop lookup, if added

### Phase 3 - Extract shared arrival presentation helpers

**Goal:** keep existing CLI output stable while making arrival formatting reusable from the TUI.

**Repo surfaces:**

- `internal/display/display.go`
- `internal/display/display_test.go`
- optional new helper file or small new package if that creates a cleaner split

**Required changes:**

1. Keep the ASCII table renderer as the non-interactive CLI presenter.
2. Extract reusable pure helpers for:
   - effective arrival time
   - arrival sorting
   - bunch-collapse
   - ETA formatting
   - status formatting
   - optional headsign truncation rules
3. Make the TUI reuse those helpers rather than duplicating formatting logic in a second place.
4. Avoid forcing the TUI through `PrintArrivals`; share data helpers, not the entire CLI rendering function.

**Acceptance criteria:**

- The retained `-stop` path still renders the same sorted/bunch-collapsed table behavior.
- TUI arrivals use the same ETA/status logic as the CLI wherever shared helpers apply.

**Tests to add or update:**

- preserve existing display behavior tests
- add tests for any extracted reusable helpers if coverage moves

### Phase 4 - Build the initial TUI package

**Goal:** create a simple, testable first-pass TUI that covers saved stops, search, and arrivals.

**Repo surfaces:**

- new `internal/tui/*.go`
- `go.mod`
- `go.sum`

**Required changes:**

1. Add the Bubble Tea dependency surface needed for the TUI implementation.
2. Keep the first TUI intentionally simple:
   - one active pane/state at a time
   - no background auto-refresh loop
   - no multi-pane dashboard
3. Structure the TUI around injected dependencies instead of global package calls.
4. Root state should cover at least:
   - bootstrap/loading
   - saved stops list
   - first-run empty state
   - search input
   - search results
   - arrivals detail
   - recoverable error state
5. First-pass keymap:
   - `up` / `down` or `j` / `k` to move
   - `enter` to open arrivals or submit a search, depending on focus
   - `/` or `s` to open search
   - `a` to save the selected search result
   - `d` to delete the selected saved stop
   - `r` to refresh arrivals
   - `esc` to back out of search/detail/error subviews
   - `q` to quit
6. Deletion should include a lightweight confirm step so `d` is not immediately destructive.
7. First-run empty state should clearly instruct the user to search and save a stop.
8. A corrupt saved-stop file should not crash the app; it should surface an explicit error view/badge and keep the failure visible.
9. Search can be explicit-submit in v1; debounce/live autocomplete is optional and not required for the migration.
10. Save/delete flows must update both in-memory model state and on-disk persistence.

**Acceptance criteria:**

- A user can launch the TUI with only `OBA_API_KEY` configured.
- A user with no saved stops can search, save one, quit, relaunch, and see it again.
- A user can open arrivals for a saved stop and manually refresh them.
- Empty search results and API failures render as explicit UI states instead of crashing.

**Tests to add or update:**

- model/update/view tests for:
  - startup bootstrap
  - empty saved-stop state
  - search flow
  - save flow
  - delete flow
  - arrivals refresh
  - error states
  - quit behavior
- avoid relying on full PTY integration tests where pure model tests are sufficient

### Phase 5 - Rewire the entrypoint and retire Wi-Fi/home-office routing

**Goal:** make the startup path match the new TUI-first product contract while preserving utility commands and direct lookup.

**Repo surfaces:**

- `main.go`
- `main_test.go`
- `internal/wifi/wifi.go`
- `internal/wifi/wifi_test.go`

**Required changes:**

1. Preserve early returns for:
   - `-version`
   - `-update`
   - `--print-config-dir`
2. Keep `-stop` + `-max-results` as the direct non-interactive path.
3. Make default no-arg execution launch the TUI.
4. Deprecate `-direction` with a custom error message instead of letting it become an unknown flag.
5. Make `-max-results` valid only with `-stop`.
6. Remove the Wi-Fi/home-office resolution branch from `main.go`.
7. Remove `resolveStop` once nothing depends on it.
8. Delete `internal/wifi` and its tests after the new startup flow no longer references it.
9. Preserve updater behavior, version injection, and config-dir printing unchanged.
10. Keep `-stop` insulated from TUI startup and saved-stop load failures.

**Acceptance criteria:**

- `-version`, `-update`, and `--print-config-dir` still exit before config or TUI startup.
- `-stop` still fetches arrivals and prints the CLI table.
- `-direction` returns a clear deprecation message.
- A broken `saved_stops.json` does not block the `-stop` path.

**Tests to add or update:**

- `main_test.go`
  - early exits still short-circuit
  - default branch launches the TUI path
  - `-stop` still prints arrivals
  - `-direction` deprecation message
  - `-max-results` validation change
  - config-load failure guidance text
- add a seam around TUI startup if needed so main-level tests can verify branching without requiring a real interactive terminal

### Phase 6 - Update setup, docs, and release artifacts

**Goal:** make installation and release documentation match the migrated product.

**Repo surfaces:**

- `.env.example`
- `setup.sh`
- `setup.ps1`
- `README.md`
- release packaging assumptions in `.github/workflows/release.yml`

**Required changes:**

1. Update `.env.example` so it reflects the new minimum setup:
   - `OBA_API_KEY`
   - comments explaining that saved stops live in `saved_stops.json`
2. Update `setup.sh` and `setup.ps1` so they:
   - still build/install the binary
   - still discover the config directory via `--print-config-dir`
   - prompt only for the API key
   - stop interviewing for home/office Wi-Fi and stop IDs
   - explain that stops are managed from inside the TUI
3. Keep the existing legacy repo-local `.env` copy behavior only if it remains useful, but do not document legacy vars as part of the active setup flow.
4. Rewrite the README sections that currently describe the Wi-Fi/home-office model:
   - Quick Start
   - Configuration
   - Manual Setup
   - Usage
   - How stop resolution works
5. Document:
   - default TUI launch
   - saved-stop workflow
   - search/save/delete/refresh keybindings
   - retained `-stop` / `-max-results` path
   - deprecation of `-direction`
6. Keep this plan current enough that another implementation agent can work directly from it.

**Acceptance criteria:**

- The setup scripts create a valid API-key-only configuration.
- The README and `.env.example` no longer describe Wi-Fi or home/office as the primary workflow.
- Release artifacts continue to ship matching docs/examples.

### Phase 7 - Validate the migration end to end

**Goal:** make the implementation reviewable and safe across the full repo surface.

**Repo validation commands:**

- `golangci-lint run`
- `go test ./...`
- `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
- `go build -o wheresmybus .`

**Additional manual smoke checks:**

1. Launch `wheresmybus` with only `OBA_API_KEY` configured and confirm the TUI opens.
2. Search for a stop and confirm results render.
3. Save a stop, quit, relaunch, and confirm the stop persists.
4. Open arrivals for a saved stop and refresh them.
5. Delete a saved stop and confirm it is gone after restart.
6. Confirm empty search results show a friendly empty state.
7. Confirm a malformed `saved_stops.json` shows a recoverable error rather than crashing.
8. Confirm `wheresmybus -stop 75403` and `wheresmybus -stop 1_75403` both still work.
9. Confirm `wheresmybus -direction home` shows the deprecation guidance.
10. Confirm `-version`, `-update`, and `--print-config-dir` still short-circuit startup.

**Cross-platform expectations:**

- CI must stay green on Linux, macOS, and Windows.
- Build artifacts must continue to work on linux/darwin/windows for amd64 and arm64.
- Setup-script behavior must be manually spot-checked on both Bash and PowerShell after the prompts change.

## Risks To Watch During Implementation

- **Config drift:** runtime config, setup scripts, `.env.example`, and README can easily diverge if they are not updated together.
- **Direct-path regressions:** the retained `-stop` path can accidentally inherit TUI-only config or persistence failures if the entrypoint split is not explicit.
- **ID normalization bugs:** search results and saved stops must keep exact OBA IDs, especially for non-`1_` agencies.
- **Formatting drift:** TUI and CLI arrivals will diverge if display logic is copied instead of shared.
- **Corrupt persistence handling:** silent overwrite is risky; the error path needs deliberate treatment.
- **Dependency creep:** Bubble Tea adds non-stdlib deps to a repo that is currently almost entirely stdlib in `internal/`; keep the added dependency surface narrow and intentional.

## Implementation Hand-off Summary

Another implementation agent should treat this migration as:

1. Relax config to API-key-only and add a safe saved-stop store.
2. Add stop-search APIs while preserving direct-stop ID normalization behavior.
3. Extract shared arrivals presentation helpers.
4. Build a simple one-pane-at-a-time TUI with search/save/delete/refresh flows.
5. Rewire `main.go` so no-arg execution launches the TUI and `-stop` remains the non-interactive path.
6. Remove the Wi-Fi/home-office workflow from runtime, docs, and setup.
7. Validate the full repo and smoke-test the user-facing paths above.
