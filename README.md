# wheresmybus

A simple CLI tool that tells you how long until your bus arrives, right from the command line.
Uses the [OneBusAway](https://pugetsound.onebusaway.org/) API for King County Metro real-time arrival data.

## Quick Start

Want the simplest option without installing Go? Download the release archive for your platform from [GitHub Releases](https://github.com/dwdankworth/wheresmybus/releases), extract it, and then follow the **Install a downloaded release** steps below. That section shows the exact folder to use on Linux, macOS, and Windows.

You can verify the installed binary with:

```sh
wheresmybus -version
```

If you'd rather build from source, use the setup scripts:

**Linux / macOS / WSL:**

```sh
./setup.sh
```

**Windows (PowerShell):**

```powershell
.\setup.ps1
```

The setup script will verify Go is installed, build the CLI, offer to add it to your PATH, and either reuse an existing repo-local `.env` or walk you through creating one in the config directory, including an optional default location for ethernet/no-Wi-Fi use.

## Configuration

`wheresmybus` looks for `.env` in this order:

1. `.env` in the current working directory (backward compatible and useful for local development)
2. Platform-specific config directory:
   - Linux: `~/.config/wheresmybus/.env`
   - macOS: `~/Library/Application Support/wheresmybus/.env`
   - Windows: `%AppData%\wheresmybus\.env`
3. Environment variables directly, if no `.env` file is found

The setup scripts write `.env` to the platform-specific config directory so the installed binary works from any directory. If you already have a legacy `.env` next to the setup script, setup reuses it by copying it into that config directory.
You can print the exact directory for your machine with `wheresmybus --print-config-dir`.

Required env vars in every setup:

- `OBA_API_KEY`
- `HOME_STOP_ID`
- `OFFICE_STOP_ID`

For stop selection, you can use either:

- Wi-Fi auto-detection with `HOME_WIFI` and `OFFICE_WIFI`
- `DEFAULT_LOCATION=home|office` as a fallback for ethernet/no-Wi-Fi devices

If `DEFAULT_LOCATION` is set, the Wi-Fi names become optional.

## Manual Setup

### 1. Get an API key

Sign up at <https://www.soundtransit.org/help-contacts/business-information/open-transit-data-otd/otd-downloads> to get your OneBusAway API key.

### 2. Find your stop IDs

Search for your home and office bus stops at <https://pugetsound.onebusaway.org/> or Google Maps. Rider-facing tools often show a bare stop code like `71335`, while the OneBusAway API uses a full stop ID like `1_71335`. `wheresmybus` accepts either format in `HOME_STOP_ID` and `OFFICE_STOP_ID`; bare numeric codes are automatically treated as Puget Sound stop IDs by prefixing them with `1_`.

### 3. Configure .env

If you'd like to use the automated setup script, you can skip this step. It will interview you to collect the needed env variables and create the `.env` file in the correct location. Create the config directory, copy the example file there, and fill in your values:

```sh
# Linux
mkdir -p ~/.config/wheresmybus
cp .env.example ~/.config/wheresmybus/.env

# macOS
mkdir -p ~/Library/Application\ Support/wheresmybus
cp .env.example ~/Library/Application\ Support/wheresmybus/.env
```

```powershell
# Windows (PowerShell)
New-Item -ItemType Directory -Force -Path "$env:AppData\wheresmybus" | Out-Null
Copy-Item .env.example "$env:AppData\wheresmybus\.env"
```

For local development, `.env` in the current working directory still works. If you set `DEFAULT_LOCATION`, you can leave `HOME_WIFI` and `OFFICE_WIFI` blank for ethernet-only setups.

Edit the copied `.env` file:

```
OBA_API_KEY=your-api-key-here
HOME_WIFI=MyHomeNetwork
OFFICE_WIFI=MyOfficeNetwork
HOME_STOP_ID=75403
OFFICE_STOP_ID=71335
# Optional fallback for ethernet/no-Wi-Fi use
DEFAULT_LOCATION=home
```

### 4. Install

#### Install a downloaded release (no Go required)

If you are new to the command line, "`PATH`" just means "a folder your terminal checks when you type a command." The easiest approach is to copy `wheresmybus` into a common command folder for your OS.

1. Open <https://github.com/dwdankworth/wheresmybus/releases>
2. Download the archive that matches your operating system and extract it
3. In the extracted folder, you should see:
   - `wheresmybus` on Linux and macOS, or `wheresmybus.exe` on Windows
   - `.env.example`
4. Copy the program into one of these common command folders:

**Linux**

```sh
mkdir -p ~/.local/bin
cp ./wheresmybus ~/.local/bin/wheresmybus
chmod +x ~/.local/bin/wheresmybus
```

**macOS**

```sh
sudo mkdir -p /usr/local/bin
sudo cp ./wheresmybus /usr/local/bin/wheresmybus
sudo chmod +x /usr/local/bin/wheresmybus
```

**Windows (PowerShell)**

```powershell
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\wheresmybus" | Out-Null
Copy-Item .\wheresmybus.exe "$env:LOCALAPPDATA\wheresmybus\wheresmybus.exe" -Force
```

5. Copy `.env.example` into the config folder for your OS and rename it to `.env`:

**Linux**

```sh
mkdir -p ~/.config/wheresmybus
cp ./.env.example ~/.config/wheresmybus/.env
```

**macOS**

```sh
mkdir -p ~/Library/Application\ Support/wheresmybus
cp ./.env.example ~/Library/Application\ Support/wheresmybus/.env
```

**Windows (PowerShell)**

```powershell
New-Item -ItemType Directory -Force -Path "$env:AppData\wheresmybus" | Out-Null
Copy-Item .\.env.example "$env:AppData\wheresmybus\.env" -Force
```

6. Edit that `.env` file and fill in your values
7. Open a new terminal window and run:

```sh
wheresmybus -version
```

If that command is still not found, the folder from step 4 is not on your `PATH` yet. In that case, either add that folder to your `PATH` or run the setup script instead, which can offer to do the `PATH` step for you automatically.

Source install:

Automatic install + PATH + .env config:

```sh
# Linux / macOS / WSL
./setup.sh

# Windows (PowerShell)
.\setup.ps1
```

Go install:

```sh
go install github.com/dwdankworth/wheresmybus@latest
```

Or build locally:

```sh
# Linux / macOS / WSL
go build -o wheresmybus .

# Windows
go build -o wheresmybus.exe .
```

## Development

CI runs lint, tests, and build. To match CI locally, install the same `golangci-lint` version pinned in `.github/workflows/ci.yml`:

```sh
GOBIN="${GOBIN:-$(go env GOPATH)/bin}" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0
```

Then run the local validation commands from the repo root:

```sh
golangci-lint run
go test ./...
go test -race -coverprofile=coverage.out ./...
go build -o wheresmybus .
```

## Usage

```sh
# Use Wi-Fi auto-detection, then fall back to DEFAULT_LOCATION if set
wheresmybus

# Show more or fewer arrivals
wheresmybus -max-results 15
wheresmybus -max-results 0

# Look up any stop directly by stop code or full stop ID
wheresmybus -stop 12345
wheresmybus -stop 1_75403

# Explicitly pick a direction
wheresmybus -direction home
wheresmybus -direction office

# Print the binary version
wheresmybus -version
```

By default, `wheresmybus` shows up to 10 arrivals. Use `-max-results <n>` to change that, or `-max-results 0` to show all arrivals after sorting and bunch-collapse deduplication.

### How stop resolution works

- `-stop 12345` or `-stop 1_75403` queries that stop directly
- `-direction home` shows arrivals for `OFFICE_STOP_ID`
- `-direction office` shows arrivals for `HOME_STOP_ID`
- Connected to `HOME_WIFI` → shows arrivals for `HOME_STOP_ID`
- Connected to `OFFICE_WIFI` → shows arrivals for `OFFICE_STOP_ID`
- If Wi-Fi does not resolve and `DEFAULT_LOCATION=home`, it uses `HOME_STOP_ID`
- If Wi-Fi does not resolve and `DEFAULT_LOCATION=office`, it uses `OFFICE_STOP_ID`
- If none of the above apply, use `-direction`

`-stop` and `-direction` cannot be used together. Bare numeric stop codes are treated as Puget Sound stop IDs and queried as `1_<code>`, so `-stop 25100` behaves the same as `-stop 1_25100`.

| Platform | Method |
|---|---|
| Linux | `nmcli` (NetworkManager) |
| macOS | `airport` utility |
| Windows / WSL | PowerShell `Get-NetConnectionProfile`, with `netsh` fallback |

### Example output

```
Arrivals for stop 1_75403:

ROUTE     DESTINATION                     ETA                 STATUS
372E      U-District Station              3 min               1 stops away
67        Northgate Station               5 min               1 stops away
67        Northgate Station               18 min              16 stops away
45        Loyal Heights Greenwood         21 min              15 stops away
372E      U-District Station              22 min              15 stops away
```
