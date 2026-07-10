#!/usr/bin/env bash
# extract-base.sh – Docker-Image extrahieren
#
# Usage: ./extract-base.sh <image-tag> [zielpfad-relativ-zu-cwd] [im-container-pfad]
#
# Beispiel:
#   ./extract-base.sh ghcr.io/org/base:20251113.135806-57d9c0b
#   → extrahiert nach ./_bases/base-20251113.135806-57d9c0b/app
#
#   ./extract-base.sh myimage:latest _bases/myimage /usr/src/app
#   → extrahiert /usr/src/app aus dem Image nach ./_bases/myimage/app

set -euo pipefail

IMG="${1:?Usage: $0 <image-tag> [dest] [container-path]}"
DEST="${2:-_bases/$(echo "$IMG" | sed -E 's|.*/([^:]+):(.*)|\1-\2|')}"
SRC="${3:-/app}"

echo "→ Image:     $IMG"
echo "→ Extract:   $SRC"
echo "→ Ziel:      $DEST"
echo ""

# 1. Pull
echo "[1/4] docker pull …"
docker pull "$IMG"

# 2. Container anlegen (nicht starten)
echo "[2/4] docker create …"
CID=$(docker create "$IMG")
trap "docker rm '$CID' >/dev/null 2>&1 || true" EXIT

# 3. Kopieren
echo "[3/4] docker cp $CID:$SRC $DEST/…"
mkdir -p "$DEST"
docker cp "$CID:$SRC" "$DEST/$(basename "$SRC")"

# 4. Aufräumen (trap erledigt das)
echo "[4/4] Cleanup"

echo ""
echo "✓ Fertig. Größe:"
du -sh "$DEST"
