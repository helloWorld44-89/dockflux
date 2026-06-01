#!/usr/bin/env bash
set -euo pipefail

REPO="helloWorld44-89/dockflux"
INSTALL_DIR="/usr/local/bin"
BIN="dockflux"

BOLD='\033[1m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
RESET='\033[0m'

step() { printf "${BLUE}==>${RESET} ${BOLD}%s${RESET}\n" "$1"; }
ok()   { printf "    ${GREEN}✓${RESET}  %s\n" "$1"; }
die()  { printf "${RED}error:${RESET} %s\n" "$1" >&2; exit 1; }

# spinner: spin <pid> <label>
spin() {
  local pid=$1 label=$2 i=0 rc=0
  local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
  while kill -0 "$pid" 2>/dev/null; do
    printf "\r    ${BLUE}%s${RESET}  %s" "${frames[$((i % 10))]}" "$label" >/dev/tty
    sleep 0.08
    i=$(( i + 1 ))
  done
  printf "\r\033[K" >/dev/tty
  wait "$pid" || rc=$?
  return $rc
}

# get <url>  — prints response body to stdout
get() {
  local url=$1
  if command -v wget &>/dev/null; then
    wget -qO- --connect-timeout=10 --tries=3 "$url"
  else
    curl -fsSL --connect-timeout 10 --retry 3 "$url"
  fi
}

# download <url> <dest> <label>
download() {
  local url=$1 dest=$2 label=$3 rc=0
  if command -v wget &>/dev/null; then
    wget -q --connect-timeout=10 --tries=3 -O "$dest" "$url" &
  else
    curl -fsSL --connect-timeout 10 --max-time 120 --retry 3 -o "$dest" "$url" &
  fi
  spin "$!" "$label" || rc=$?
  [ $rc -eq 0 ] || die "download failed (exit $rc): $url"
  ok "$label"
}

# ── detect platform ────────────────────────────────────────────────────────────

step "Detecting platform"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) die "unsupported OS: $OS" ;;
esac

ASSET="${BIN}-${OS}-${ARCH}"
ok "$OS / $ARCH"

# ── resolve latest tag ─────────────────────────────────────────────────────────

step "Fetching latest release"

TAG="$(get "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')" \
  || die "could not reach api.github.com"

[ -n "$TAG" ] || die "could not parse release tag — check https://github.com/${REPO}/releases"
ok "$TAG"

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# ── download ───────────────────────────────────────────────────────────────────

step "Downloading"
download "${BASE_URL}/${ASSET}"      "${TMP}/${ASSET}"      "$ASSET"
download "${BASE_URL}/checksums.txt" "${TMP}/checksums.txt" "checksums.txt"

# ── verify ─────────────────────────────────────────────────────────────────────

step "Verifying"
cd "$TMP"
if command -v sha256sum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sha256sum --check --status || die "checksum mismatch"
elif command -v shasum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sed 's/  / */' | shasum -a 256 --check --status || die "checksum mismatch"
else
  printf "    (skipped — no sha256 tool found)\n"
fi
cd - >/dev/null
ok "SHA256 verified"

# ── install ────────────────────────────────────────────────────────────────────

step "Installing"
chmod +x "${TMP}/${ASSET}"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/${ASSET}" "${INSTALL_DIR}/${BIN}"
else
  sudo mv "${TMP}/${ASSET}" "${INSTALL_DIR}/${BIN}"
fi
ok "${INSTALL_DIR}/${BIN}"

VERSION="$("${INSTALL_DIR}/${BIN}" --version 2>&1 | awk '{print $NF}')"
printf "\n${GREEN}${BOLD}  dockflux %s installed successfully${RESET}\n" "$VERSION"
printf "  Run ${BOLD}dockflux init${RESET} to get started.\n\n"
