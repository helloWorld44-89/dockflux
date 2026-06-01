#!/usr/bin/env bash
set -euo pipefail

REPO="helloWorld44-89/dockflux"
INSTALL_DIR="/usr/local/bin"
BIN="dockflux"

# --- ui ---

BOLD='\033[1m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
RESET='\033[0m'

say()  { printf "%b" "$@" >/dev/tty; }
step() { say "\n${BLUE}==>${RESET} ${BOLD}$1${RESET}\n"; }
ok()   { say "    ${GREEN}✓${RESET}  $1\n"; }
die()  { say "    ${RED}✗${RESET}  $1\n"; exit 1; }

spin() {
  local pid=$1 msg=$2 i=0 rc=0
  local f=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
  while kill -0 "$pid" 2>/dev/null; do
    say "\r    ${BLUE}${f[$((i % 10))]}${RESET}  $msg"
    sleep 0.08
    i=$(( i + 1 ))
  done
  say "\r\033[K"
  wait "$pid" || rc=$?
  return $rc
}

fetch() {
  local url="$1" dest="$2" msg="$3" rc=0
  if command -v wget &>/dev/null; then
    wget -q --connect-timeout=10 --tries=3 --waitretry=2 -O "$dest" "$url" &
  else
    curl -fsSL --connect-timeout 10 --max-time 120 --retry 3 --retry-delay 2 \
      -o "$dest" "$url" &
  fi
  spin "$!" "$msg" || rc=$?
  [ $rc -eq 0 ] || die "Failed to download: $url (exit $rc)"
  ok "$msg"
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

fetch "https://api.github.com/repos/${REPO}/releases/latest" "${TMP}/release.json" "api.github.com"

TAG="$(grep '"tag_name"' "${TMP}/release.json" \
  | head -1 \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"

[ -n "$TAG" ] || die "Could not parse release tag from response."
ok "$TAG"

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

# --- download ---

step "Downloading"

fetch "${BASE_URL}/${ASSET}"      "${TMP}/${ASSET}"        "$ASSET"
fetch "${BASE_URL}/checksums.txt" "${TMP}/checksums.txt"   "checksums.txt"

# --- verify ---

step "Verifying checksum"

cd "$TMP"
if command -v sha256sum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sha256sum --check --status || die "Checksum mismatch."
elif command -v shasum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sed 's/  / */' | shasum -a 256 --check --status || die "Checksum mismatch."
else
  say "    (skipped — no sha256 tool found)\n"
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

VERSION="$("${INSTALL_DIR}/${BIN}" --version 2>&1 | awk '{print $NF}')"
say "\n${GREEN}${BOLD}  dockflux ${VERSION} installed successfully${RESET}\n"
say "  Run ${BOLD}dockflux init${RESET} to get started.\n\n"
