#!/usr/bin/env bash
set -euo pipefail

REPO="helloWorld44-89/dockflux"
INSTALL_DIR="/usr/local/bin"
BIN="dockflux"

# --- ui ---

BOLD='\033[1m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RESET='\033[0m'

tty() { printf "%b" "$@" >/dev/tty; }
step() { tty "\n${BLUE}==>${RESET} ${BOLD}$1${RESET}\n"; }
ok()   { tty "    ${GREEN}✓${RESET}  $1\n"; }
die()  { tty "\n    ✗  $1\n"; exit 1; }

spin() {
  local pid=$1 msg=$2 i=0
  local f=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
  while kill -0 "$pid" 2>/dev/null; do
    tty "\r    ${BLUE}${f[$((i % 10))]}${RESET}  $msg"
    sleep 0.08
    i=$(( i + 1 ))
  done
  tty "\r    ${GREEN}✓${RESET}  $msg\n"
}

fetch() {
  local url="$1" dest="$2" msg="$3"
  if command -v wget &>/dev/null; then
    wget -q --connect-timeout=10 --tries=3 --waitretry=2 -O "$dest" "$url" &
  else
    curl -fsSL --connect-timeout 10 --max-time 120 --retry 3 --retry-delay 2 \
      -o "$dest" "$url" &
  fi
  local pid=$!
  spin "$pid" "$msg"
  wait "$pid"
}

# --- detect platform ---

step "Detecting platform"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) die "Unsupported OS: $OS" ;;
esac

ASSET="${BIN}-${OS}-${ARCH}"
ok "$OS/$ARCH"

# --- resolve latest version ---

step "Fetching latest release"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fetch "https://api.github.com/repos/${REPO}/releases/latest" "${TMP}/release.json" "Checking GitHub..."

TAG="$(grep '"tag_name"' "${TMP}/release.json" \
  | head -1 \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"

[ -n "$TAG" ] || die "Could not parse latest release tag."
ok "$TAG"

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

# --- download ---

step "Downloading"

fetch "${BASE_URL}/${ASSET}"     "${TMP}/${ASSET}"       "$ASSET"
fetch "${BASE_URL}/checksums.txt" "${TMP}/checksums.txt" "checksums.txt"

# --- verify ---

step "Verifying checksum"

cd "$TMP"
if command -v sha256sum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sha256sum --check --status || die "Checksum mismatch."
elif command -v shasum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sed 's/  / */' | shasum -a 256 --check --status || die "Checksum mismatch."
else
  tty "    (skipped — no sha256 tool found)\n"
fi
cd - >/dev/null
ok "SHA256 verified"

chmod +x "${TMP}/${ASSET}"
mv "${TMP}/${ASSET}" "${TMP}/${BIN}"

# --- install ---

step "Installing"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"
else
  sudo mv "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"
fi

ok "${INSTALL_DIR}/${BIN}"

tty "\n${GREEN}${BOLD}  dockflux $("${INSTALL_DIR}/${BIN}" --version 2>&1 | awk '{print $NF}') installed successfully${RESET}\n"
tty "  Run ${BOLD}dockflux init${RESET} to get started.\n\n"
