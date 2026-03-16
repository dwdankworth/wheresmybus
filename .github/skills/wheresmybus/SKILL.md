---
name: wheresmybus
description: Check real-time King County Metro bus arrivals from the command line
---

## wheresmybus CLI

`wheresmybus` is a CLI tool for real-time King County Metro bus arrivals via the OneBusAway Puget Sound API. It is installed and on PATH.

### Usage
- `wheresmybus` — auto-detect stop via Wi-Fi SSID or DEFAULT_LOCATION fallback
- `wheresmybus -stop <id>` — query a specific stop (bare code like `12345` or full ID like `1_75403`)
- `wheresmybus -direction home|office` — explicitly pick home or office stop
- `wheresmybus -version` — print version
- `wheresmybus --print-config-dir` — show config directory path

`-stop` and `-direction` cannot be combined.

### Stop resolution order
1. `-stop` flag → query that stop directly
2. `-direction home` → OFFICE_STOP_ID; `-direction office` → HOME_STOP_ID
3. Wi-Fi auto-detect: HOME_WIFI → HOME_STOP_ID, OFFICE_WIFI → OFFICE_STOP_ID
4. DEFAULT_LOCATION fallback (home|office)

### Configuration
Config is a `.env` file searched in order: CWD, then platform config dir (Linux: `~/.config/wheresmybus/`, macOS: `~/Library/Application Support/wheresmybus/`, Windows: `%AppData%\wheresmybus\`), then env vars directly. Required: `OBA_API_KEY`, `HOME_STOP_ID`, `OFFICE_STOP_ID`. Optional: `HOME_WIFI`, `OFFICE_WIFI`, `DEFAULT_LOCATION`.
