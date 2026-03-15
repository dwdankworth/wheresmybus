#!/usr/bin/env bash
set -euo pipefail

# ---------- colors (disabled when not a terminal) ----------
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'  RED='\033[0;31m'  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'   BOLD='\033[1m'    RESET='\033[0m'
else
  GREEN='' RED='' YELLOW='' BLUE='' BOLD='' RESET=''
fi

info()  { printf "${BLUE}i${RESET}  %s\n" "$*"; }
ok()    { printf "${GREEN}+${RESET}  %s\n" "$*"; }
warn()  { printf "${YELLOW}!${RESET}  %s\n" "$*"; }
fail()  { printf "${RED}x${RESET}  %s\n" "$*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ---------- 1. Check Go is installed ----------
info "Checking for Go..."
if ! command -v go &>/dev/null; then
  fail "Go is not installed. Install it from https://go.dev/dl/ and re-run this script."
fi
ok "Go found: $(go version)"

# ---------- 2. Enforce minimum Go version ----------
REQUIRED_MAJOR=1
REQUIRED_MINOR=25
REQUIRED_PATCH=8

go_version_string="$(go version | grep -oP 'go\K[0-9]+\.[0-9]+(\.[0-9]+)?')"
IFS='.' read -r got_major got_minor got_patch <<< "$go_version_string"
got_patch="${got_patch:-0}"

version_ok=false
if (( got_major > REQUIRED_MAJOR )); then
  version_ok=true
elif (( got_major == REQUIRED_MAJOR )); then
  if (( got_minor > REQUIRED_MINOR )); then
    version_ok=true
  elif (( got_minor == REQUIRED_MINOR && got_patch >= REQUIRED_PATCH )); then
    version_ok=true
  fi
fi

if [[ "$version_ok" != true ]]; then
  fail "Go ${REQUIRED_MAJOR}.${REQUIRED_MINOR}.${REQUIRED_PATCH}+ is required (found ${go_version_string}). Please upgrade: https://go.dev/dl/"
fi
ok "Go version ${go_version_string} meets minimum requirement (${REQUIRED_MAJOR}.${REQUIRED_MINOR}.${REQUIRED_PATCH})"

# ---------- 3. Build the binary ----------
info "Building wheresmybus..."
cd "$SCRIPT_DIR"
go build -o wheresmybus .
ok "Built ./wheresmybus"

# ---------- 4. Offer to install to PATH ----------
INSTALL_DIR="$HOME/.local/bin"

printf '\n%bAdd wheresmybus to your PATH?%b\n' "$BOLD" "$RESET"
printf '  This copies the binary to %b%s%b so you can run %bwheresmybus%b from anywhere.\n' "$BLUE" "$INSTALL_DIR" "$RESET" "$BOLD" "$RESET"
read -rp "  Install to PATH? [Y/n] " answer
answer="${answer:-Y}"

if [[ "$answer" =~ ^[Yy] ]]; then
  mkdir -p "$INSTALL_DIR"

  if [[ -f "$INSTALL_DIR/wheresmybus" ]]; then
    read -rp "  ${INSTALL_DIR}/wheresmybus already exists. Overwrite? [Y/n] " overwrite
    overwrite="${overwrite:-Y}"
    if [[ ! "$overwrite" =~ ^[Yy] ]]; then
      warn "Skipped PATH installation."
    else
      cp wheresmybus "$INSTALL_DIR/wheresmybus"
      ok "Updated ${INSTALL_DIR}/wheresmybus"
    fi
  else
    cp wheresmybus "$INSTALL_DIR/wheresmybus"
    ok "Installed to ${INSTALL_DIR}/wheresmybus"
  fi

  # Ensure ~/.local/bin is on PATH in the user's shell profile
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"
    case "$SHELL_NAME" in
      zsh)  PROFILE="$HOME/.zshrc" ;;
      bash) PROFILE="$HOME/.bashrc" ;;
      *)    PROFILE="$HOME/.profile" ;;
    esac

    # shellcheck disable=SC2016
    export_line='export PATH="$HOME/.local/bin:$PATH"'
    if ! grep -qF '.local/bin' "$PROFILE" 2>/dev/null; then
      printf '\n# Added by wheresmybus setup\n%s\n' "$export_line" >> "$PROFILE"
      ok "Added ${INSTALL_DIR} to PATH in ${PROFILE}"
      warn "Run 'source ${PROFILE}' or open a new terminal for changes to take effect."
    else
      ok "${INSTALL_DIR} is already referenced in ${PROFILE}"
    fi
  else
    ok "${INSTALL_DIR} is already on PATH"
  fi
else
  info "Skipped PATH installation. You can run ./wheresmybus from this directory."
fi

# ---------- 5. Configure .env ----------
printf '\n%bConfigure .env%b\n' "$BOLD" "$RESET"

if ! CONFIG_DIR="$(./wheresmybus --print-config-dir)"; then
  fail "Could not determine the config directory."
fi
ENV_FILE="$CONFIG_DIR/.env"
LEGACY_ENV_FILE="$SCRIPT_DIR/.env"

if [[ -f "$ENV_FILE" ]]; then
  read -rp "  .env already exists. Reconfigure? [y/N] " reconfig
  reconfig="${reconfig:-N}"
  if [[ ! "$reconfig" =~ ^[Yy] ]]; then
    ok "Keeping existing .env"
    printf '\n%b%bSetup complete!%b\n' "$GREEN" "$BOLD" "$RESET"
    printf 'Configuration is stored in %b%s%b.\n' "$BOLD" "$ENV_FILE" "$RESET"
    printf 'Run %bwheresmybus%b to see your next bus.\n' "$BOLD" "$RESET"
    exit 0
  fi
fi

if [[ -f "$LEGACY_ENV_FILE" ]]; then
  mkdir -p "$CONFIG_DIR"
  cp "$LEGACY_ENV_FILE" "$ENV_FILE"
  ok "Copied existing .env from ${LEGACY_ENV_FILE} to ${ENV_FILE}"
  printf '\n%b%bSetup complete!%b\n' "$GREEN" "$BOLD" "$RESET"
  printf 'Configuration is stored in %b%s%b.\n' "$BOLD" "$ENV_FILE" "$RESET"
  printf 'Run %bwheresmybus%b to see your next bus.\n' "$BOLD" "$RESET"
  exit 0
fi

info "Let's configure your settings."
printf '\n'

# API key
printf '  %bOneBusAway API Key%b\n' "$BOLD" "$RESET"
printf '  Sign up at %bhttps://www.pugetsound.onebusaway.org/p/sign-up%b\n' "$BLUE" "$RESET"
read -rp "  API key: " oba_api_key
if [[ -z "$oba_api_key" ]]; then
  fail "API key is required."
fi

# Default location
default_location=""
printf '\n  %bDefault location for ethernet/no-WiFi use%b\n' "$BOLD" "$RESET"
read -rp "  Set a default location? [y/N] " configure_default_location
configure_default_location="${configure_default_location:-N}"

if [[ "$configure_default_location" =~ ^[Yy] ]]; then
  while true; do
    read -rp "  Default location [home/office]: " default_location
    default_location="${default_location,,}"
    if [[ "$default_location" == "home" || "$default_location" == "office" ]]; then
      break
    fi
    warn "Please enter 'home' or 'office'."
  done
fi

# WiFi names
if [[ -n "$default_location" ]]; then
  printf '\n  %bWiFi network names%b (optional when a default location is set)\n' "$BOLD" "$RESET"
  printf '  Leave blank to skip WiFi auto-detection on this device.\n'
else
  printf '\n  %bWiFi network names%b (required unless you set a default location)\n' "$BOLD" "$RESET"
fi
read -rp "  Home WiFi SSID: " home_wifi

read -rp "  Office WiFi SSID: " office_wifi

if [[ -z "$default_location" ]] && { [[ -z "$home_wifi" ]] || [[ -z "$office_wifi" ]]; }; then
  fail "HOME_WIFI and OFFICE_WIFI are required unless you set a default location."
fi

# Stop IDs
printf '\n  %bBus stop IDs%b\n' "$BOLD" "$RESET"
printf '  Find yours at %bhttps://pugetsound.onebusaway.org%b (e.g. %b1_75403%b)\n' "$BLUE" "$RESET" "$BOLD" "$RESET"
read -rp "  Home stop ID: " home_stop_id
read -rp "  Office stop ID: " office_stop_id

mkdir -p "$CONFIG_DIR"
cat > "$ENV_FILE" <<EOF
OBA_API_KEY=${oba_api_key}
HOME_WIFI=${home_wifi}
OFFICE_WIFI=${office_wifi}
HOME_STOP_ID=${home_stop_id}
OFFICE_STOP_ID=${office_stop_id}
EOF

if [[ -n "$default_location" ]]; then
  printf 'DEFAULT_LOCATION=%s\n' "$default_location" >> "$ENV_FILE"
fi

ok "Wrote ${ENV_FILE}"

# ---------- Done ----------
printf '\n%b%bSetup complete!%b\n' "$GREEN" "$BOLD" "$RESET"
printf 'Configuration is stored in %b%s%b.\n' "$BOLD" "$ENV_FILE" "$RESET"
printf 'Run %bwheresmybus%b to see your next bus.\n' "$BOLD" "$RESET"
