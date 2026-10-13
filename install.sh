#!/bin/sh
# Install Office Brains (brains) from GitHub Releases.
#   curl -fsSL https://raw.githubusercontent.com/DERYK-RYLOY/office-brains/main/install.sh | sh
set -eu
TAG="v0.1.0"
REPO="DERYK-RYLOY/office-brains"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] && ARCH="amd64"
ASSET="brains-${OS}-${ARCH}"
DESTDIR="$HOME/.office-brains/bin"
mkdir -p "$DESTDIR"
curl -fsSL "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}" -o "${DESTDIR}/brains"
chmod +x "${DESTDIR}/brains"
"${DESTDIR}/brains" doctor 2>&1 | head -3
echo "Installed to ${DESTDIR}/brains. Add ${DESTDIR} to PATH to run 'brains' anywhere."
