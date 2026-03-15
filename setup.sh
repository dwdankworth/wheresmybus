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

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  read -rp "  .env already exists. Reconfigure? [y/N] " reconfig
  reconfig="${reconfig:-N}"
  if [[ ! "$reconfig" =~ ^[Yy] ]]; then
    ok "Keeping existing .env"
    printf '\n%b%bSetup complete!%b\n' "$GREEN" "$BOLD" "$RESET"
    printf 'Run %bwheresmybus%b to see your next bus.\n' "$BOLD" "$RESET"
    exit 0
  fi
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

# Home wifi
printf '\n  %bHome WiFi network name%b (used for auto-detecting direction)\n' "$BOLD" "$RESET"
read -rp "  Home WiFi SSID: " home_wifi

# Office wifi
printf '\n  %bOffice WiFi network name%b\n' "$BOLD" "$RESET"
read -rp "  Office WiFi SSID: " office_wifi

# Stop IDs
printf '\n  %bBus stop IDs%b\n' "$BOLD" "$RESET"
printf '  Find yours at %bhttps://pugetsound.onebusaway.org%b (e.g. %b1_75403%b)\n' "$BLUE" "$RESET" "$BOLD" "$RESET"
read -rp "  Home stop ID: " home_stop_id
read -rp "  Office stop ID: " office_stop_id

cat > "$SCRIPT_DIR/.env" <<EOF
OBA_API_KEY=${oba_api_key}
HOME_WIFI=${home_wifi}
OFFICE_WIFI=${office_wifi}
HOME_STOP_ID=${home_stop_id}
OFFICE_STOP_ID=${office_stop_id}
EOF

ok "Wrote .env"

# ---------- Done ----------
printf '\n%b%bSetup complete!%b\n' "$GREEN" "$BOLD" "$RESET"
printf 'Run %bwheresmybus%b to see your next bus.\n' "$BOLD" "$RESET"
