#!/usr/bin/env bash
# Builds a drag-to-Applications DMG from an existing .app bundle.
set -euo pipefail

APP_NAME="${1:-geckty}"
ARCH="${2:-arm64}"
BIN_DIR="${3:-bin}"

DMG_NAME="${APP_NAME}-macos-${ARCH}.dmg"
STAGING="${BIN_DIR}/.dmg-staging-${ARCH}"
APP_PATH="${BIN_DIR}/${APP_NAME}.app"
OUTPUT="${BIN_DIR}/${DMG_NAME}"

if [[ ! -d "${APP_PATH}" ]]; then
  echo "error: ${APP_PATH} not found — run darwin:package first" >&2
  exit 1
fi

rm -rf "${STAGING}" "${OUTPUT}"
mkdir -p "${STAGING}"
ditto "${APP_PATH}" "${STAGING}/${APP_NAME}.app"
ln -s /Applications "${STAGING}/Applications"

hdiutil create \
  -volname "${APP_NAME}" \
  -srcfolder "${STAGING}" \
  -ov \
  -format UDZO \
  "${OUTPUT}"

rm -rf "${STAGING}" "${APP_PATH}"
echo "created ${OUTPUT}"
