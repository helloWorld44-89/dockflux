#!/usr/bin/env bash
set -euo pipefail

REPO="helloWorld44-89/dockflux"
INSTALL_DIR="/usr/local/bin"
BIN="dockflux"

# --- detect OS and arch ---

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

ASSET="${BIN}-${OS}-${ARCH}"

# --- resolve latest version ---

echo "Fetching latest release..."
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"

if [ -z "$TAG" ]; then
  echo "Could not determine latest release tag."
  exit 1
fi

echo "Latest: $TAG"

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

# --- download to temp dir ---

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

download() {
  local url="$1" dest="$2" silent="$3"
  if command -v wget &>/dev/null; then
    if [ "$silent" = "true" ]; then
      wget -q -O "$dest" "$url"
    else
      wget --show-progress -q -O "$dest" "$url" 2>/dev/tty
    fi
  else
    if [ "$silent" = "true" ]; then
      curl -fsSL --connect-timeout 10 --max-time 60 --retry 3 --retry-delay 2 -o "$dest" "$url"
    else
      curl -fL --connect-timeout 10 --max-time 120 --retry 3 --retry-delay 2 -o "$dest" "$url" 2>/dev/tty
    fi
  fi
}

echo "Downloading $ASSET..."
download "${BASE_URL}/${ASSET}" "${TMP}/${ASSET}" "false"
download "${BASE_URL}/checksums.txt" "${TMP}/checksums.txt" "true"

# --- verify checksum ---

echo "Verifying checksum..."
cd "$TMP"

if command -v sha256sum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sha256sum --check --status
elif command -v shasum &>/dev/null; then
  grep "${ASSET}" checksums.txt | sed 's/  / */' | shasum -a 256 --check --status
else
  echo "Warning: no sha256 tool found, skipping checksum verification."
fi

cd - >/dev/null

chmod +x "${TMP}/${ASSET}"
mv "${TMP}/${ASSET}" "${TMP}/${BIN}"

# --- install ---

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"
fi

echo ""
echo "Installed: $(${INSTALL_DIR}/${BIN} --version)"
echo "Run 'dockflux init' to get started."
