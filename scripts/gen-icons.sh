#!/usr/bin/env bash
# Generates every platform's packaged app icon from assets/icon.png (the
# real geckty mark — a gecko peeking over a terminal prompt), also embedded
# at runtime as the window icon (see assets/assets.go and
# internal/ui/app/app.go's loadAppIcon). Requires macOS's sips/iconutil
# (used for resizing and .icns packing); run this on a Mac, not in CI, and
# commit the results — build/{darwin,windows}/*.ic{ns,o} are checked in,
# not regenerated at build time.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v sips >/dev/null 2>&1 || ! command -v iconutil >/dev/null 2>&1; then
  echo "error: sips and iconutil (macOS only) are required to regenerate icons" >&2
  exit 1
fi

SRC="assets/icon.png"

echo "==> Building macOS .iconset"
ICONSET="$(mktemp -d)/geckty.iconset"
mkdir -p "$ICONSET"
for px in 16 32 128 256 512; do
  sips -z "$px" "$px" "$SRC" --out "$ICONSET/icon_${px}x${px}.png" >/dev/null
  double=$((px * 2))
  sips -z "$double" "$double" "$SRC" --out "$ICONSET/icon_${px}x${px}@2x.png" >/dev/null
done
mkdir -p build/darwin
iconutil -c icns "$ICONSET" -o build/darwin/icons.icns
rm -rf "$(dirname "$ICONSET")"
echo "wrote build/darwin/icons.icns"

echo "==> Building Windows .ico"
mkdir -p build/windows
TMP_ICO="$(mktemp -d)"
for px in 16 32 48 256; do
  sips -z "$px" "$px" "$SRC" --out "$TMP_ICO/icon-${px}.png" >/dev/null
done
go run ./scripts/gen-icon/ico -out build/windows/icon.ico \
  "$TMP_ICO/icon-16.png" "$TMP_ICO/icon-32.png" "$TMP_ICO/icon-48.png" "$TMP_ICO/icon-256.png"
rm -rf "$TMP_ICO"

echo "==> Building Linux icon"
mkdir -p build/linux
sips -z 256 256 "$SRC" --out build/linux/icon-256.png >/dev/null
echo "wrote build/linux/icon-256.png"

echo "Done."
