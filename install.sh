#!/usr/bin/env bash
# Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
# Licensed under the MIT License. See LICENSE file in project root for details.
# Install script for kctl-tui.
# Downloads the latest GitHub release binary matching the current OS/arch
# and installs it to /usr/local/bin (or $INSTALL_DIR if set).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/skoelle/kctl-tui/main/install.sh | bash
#
# Requires: curl, a supported OS (linux, darwin).
# Native Windows is not supported by this script; use WSL, or download the
# .exe asset manually from the Releases page.

set -euo pipefail

REPO="skoelle/kctl-tui"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BIN_NAME="kctl-tui"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

case "$os" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $os. On Windows, use WSL or download the .exe from the Releases page." >&2
    exit 1
    ;;
esac

echo "Detecting latest release for $REPO ..."

release_json="$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest")" || {
  echo "Failed to reach the GitHub API (network error). Check your internet connection and try again." >&2
  exit 1
}

echo "Received ${#release_json} bytes from the GitHub API."

if echo "$release_json" | grep -q '"message"[[:space:]]*:[[:space:]]*"Not Found"'; then
  echo "" >&2
  echo "Could not find a published release for ${REPO}." >&2
  echo "This usually means no release has been tagged yet." >&2
  echo "" >&2
  echo "Options:" >&2
  echo "  1) Ask the maintainer to push a version tag (e.g. 'git tag v0.1.0 && git push origin v0.1.0')," >&2
  echo "     which triggers the release build via GitHub Actions." >&2
  echo "  2) Build from source instead:" >&2
  echo "       git clone https://github.com/${REPO}.git" >&2
  echo "       cd $(basename "$REPO")" >&2
  echo "       go build -o ${BIN_NAME} ./cmd/${BIN_NAME}" >&2
  echo "       sudo mv ${BIN_NAME} ${INSTALL_DIR}/" >&2
  exit 1
fi

if echo "$release_json" | grep -qi 'rate limit exceeded'; then
  echo "GitHub API rate limit exceeded. Wait a bit and try again, or authenticate with a GitHub token." >&2
  exit 1
fi

# Note: '|| true' below prevents 'set -o pipefail' + 'set -e' from aborting the
# script silently if grep finds no match; the emptiness check right after
# gives a proper diagnostic instead.
latest_tag="$(echo "$release_json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')" || true

if [ -z "$latest_tag" ]; then
  echo "Could not parse the latest release tag from the GitHub API response." >&2
  echo "Raw response (truncated):" >&2
  echo "$release_json" | head -c 800 >&2
  echo "" >&2
  exit 1
fi

echo "Latest release tag: ${latest_tag}"

asset="kctl-tui-${os}-${arch}"
url="https://github.com/${REPO}/releases/download/${latest_tag}/${asset}"

echo "Downloading ${asset} (${latest_tag}) ..."
tmp_file="$(mktemp)"
curl -fsSL "$url" -o "$tmp_file"
chmod +x "$tmp_file"

if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp_file" "${INSTALL_DIR}/${BIN_NAME}"
else
  echo "Elevated permissions required to write to ${INSTALL_DIR}"
  sudo mv "$tmp_file" "${INSTALL_DIR}/${BIN_NAME}"
fi

echo "Installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
echo "Done. Run '${BIN_NAME}' to get started."
