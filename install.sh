#!/usr/bin/env bash
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

http_status="$(curl -sSL -o /tmp/kctl-tui-release.json -w '%{http_code}' "https://api.github.com/repos/${REPO}/releases/latest")"

if [ "$http_status" != "200" ]; then
  echo "" >&2
  echo "Could not find a published release for ${REPO} (HTTP ${http_status})." >&2
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
  rm -f /tmp/kctl-tui-release.json
  exit 1
fi

latest_tag="$(grep -m1 '"tag_name"' /tmp/kctl-tui-release.json | sed -E 's/.*"([^"]+)".*/\1/')"
rm -f /tmp/kctl-tui-release.json

if [ -z "$latest_tag" ]; then
  echo "Could not parse the latest release tag from the GitHub API response." >&2
  exit 1
fi

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
"${INSTALL_DIR}/${BIN_NAME}" --help >/dev/null 2>&1 || true
echo "Done. Run '${BIN_NAME}' to get started."
