#!/usr/bin/env bash
# Generates every platform's packaged app icon from the single placeholder
# source image (scripts/gen-icon), which is not final design work — see
# that command's doc comment. Requires macOS's sips/iconutil (used for
# resizing and .icns packing); run this on a Mac, not in CI, and commit
# the results — build/{darwin,windows}/*.ic{ns,o} and assets/*.png are
# checked in, not regenerated at build time.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v sips >/dev/null 2>&1 || ! command -v iconutil >/dev/null 2>&1; then
  echo "error: sips and iconutil (macOS only) are required to regenerate icons" >&2
  exit 1
fi

echo "==> Generating base placeholder icon"
go run ./scripts/gen-icon

echo "==> Building macOS .iconset"
ICONSET="$(mktemp -d)/geckty.iconset"
mkdir -p "$ICONSET"
for px in 16 32 128 256 512; do
  sips -z "$px" "$px" assets/icon-1024.png --out "$ICONSET/icon_${px}x${px}.png" >/dev/null
  double=$((px * 2))
  sips -z "$double" "$double" assets/icon-1024.png --out "$ICONSET/icon_${px}x${px}@2x.png" >/dev/null
done
mkdir -p build/darwin
iconutil -c icns "$ICONSET" -o build/darwin/icons.icns
rm -rf "$(dirname "$ICONSET")"
echo "wrote build/darwin/icons.icns"

echo "==> Building Windows .ico"
mkdir -p build/windows
TMP_ICO="$(mktemp -d)"
for px in 16 32 48 256; do
  sips -z "$px" "$px" assets/icon-1024.png --out "$TMP_ICO/icon-${px}.png" >/dev/null
done
go run ./scripts/gen-icon/ico -out build/windows/icon.ico \
  "$TMP_ICO/icon-16.png" "$TMP_ICO/icon-32.png" "$TMP_ICO/icon-48.png" "$TMP_ICO/icon-256.png"
rm -rf "$TMP_ICO"

echo "==> Building Linux icon"
mkdir -p build/linux
sips -z 256 256 assets/icon-1024.png --out build/linux/icon-256.png >/dev/null
echo "wrote build/linux/icon-256.png"

echo "Done."
