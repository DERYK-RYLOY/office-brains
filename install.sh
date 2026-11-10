#!/bin/sh
# Install Office Brains (brains) from GitHub Releases, verifying SHA-256.
#
#   curl -fsSL https://raw.githubusercontent.com/DERYK-RYLOY/office-brains/main/install.sh | sh
#
# Override the release tag with BRAINS_TAG (default: v0.2.0).
# The binary is rejected and not installed if its hash does not match
# SHA256SUMS.txt from the same release.
set -e

TAG="${BRAINS_TAG:-v0.2.0}"
REPO="DERYK-RYLOY/office-brains"
ASSET="brains-linux-amd64"
DEST_DIR="$HOME/.office-brains/bin"
DEST="$DEST_DIR/brains"
BASE="https://github.com/$REPO/releases/download/$TAG"

mkdir -p "$DEST_DIR"

echo "Fetching SHA256SUMS.txt for $TAG..."
SUMS=$(curl -fsSL "$BASE/SHA256SUMS.txt") || { echo "could not download SHA256SUMS.txt from $TAG - aborting." >&2; exit 1; }

EXPECTED=$(printf '%s\n' "$SUMS" | grep -F "$ASSET" | head -n1 | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "SHA256SUMS.txt from $TAG does not list $ASSET - aborting." >&2
  exit 1
fi

echo "Downloading $ASSET..."
curl -fsSL -o "$DEST.tmp" "$BASE/$ASSET"

ACTUAL=$(sha256sum "$DEST.tmp" | awk '{print $1}')
if [ "$ACTUAL" != "$EXPECTED" ]; then
  rm -f "$DEST.tmp"
  echo "checksum mismatch: expected $EXPECTED, got $ACTUAL. The binary was NOT installed." >&2
  exit 1
fi

mv "$DEST.tmp" "$DEST"
chmod +x "$DEST"
echo "sha256 verified."
"$DEST" doctor | head -n 5 || true
echo "Installed to $DEST. Add $DEST_DIR to PATH to run 'brains' anywhere."
