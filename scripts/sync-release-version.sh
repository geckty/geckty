#!/usr/bin/env bash
# Sync release version from tag into build assets (Info.plist, nfpm.yaml,
# nsis tools.nsh).
#
# Usage:
#   VERSION=1.2.3 bash scripts/sync-release-version.sh
#   bash scripts/sync-release-version.sh   # uses GITHUB_REF_NAME or 0.0.0-dev
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

raw="${VERSION:-${GITHUB_REF_NAME:-}}"
raw="${raw#v}"

if [[ -z "$raw" || "$raw" == "main" || "$raw" == "master" ]]; then
  raw="0.0.0-dev"
fi

echo "Syncing version: ${raw}"

sed_inplace() {
  # macOS's BSD sed requires an explicit (possibly empty) backup suffix
  # argument after -i; GNU sed (Linux CI runners) errors if given one.
  if [[ "$(uname -s)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# ── macOS Info.plist ────────────────────────────────────────────────────
# Targets only the <string> immediately after CFBundleVersion/
# CFBundleShortVersionString's own <key> line (sed's `n` reads the next
# line before substituting on it), rather than any x.y.z-shaped string
# anywhere in the file — a first version of this script used a plain
# global match and silently also rewrote LSMinimumSystemVersion's "12.0.0"
# (a real, non-version macOS deployment-target field that just happens to
# look like a version number too) to the release tag. Found by actually
# running this script and diffing the result, not by inspection.
PLIST="build/darwin/Info.plist"
if [[ -f "$PLIST" ]]; then
  sed_inplace \
    -e "/<key>CFBundleVersion<\\/key>/{n;s|<string>[^<]*</string>|<string>${raw}</string>|;}" \
    -e "/<key>CFBundleShortVersionString<\\/key>/{n;s|<string>[^<]*</string>|<string>${raw}</string>|;}" \
    "$PLIST"
  echo "Updated ${PLIST}"
fi

# ── Linux nfpm.yaml ─────────────────────────────────────────────────────
NFPM="build/linux/nfpm/nfpm.yaml"
if [[ -f "$NFPM" ]]; then
  sed_inplace \
    -e "s|^version: \"[^\"]*\"|version: \"${raw}\"|" \
    "$NFPM"
  echo "Updated ${NFPM}"
fi

# ── Windows NSIS tools.nsh ──────────────────────────────────────────────
NSH="build/windows/nsis/tools.nsh"
if [[ -f "$NSH" ]]; then
  sed_inplace "s/!define INFO_PRODUCTVERSION \"[^\"]*\"/!define INFO_PRODUCTVERSION \"${raw}\"/" "$NSH"
  echo "Updated ${NSH}"
fi

echo "Version sync complete: ${raw}"
