#!/usr/bin/env bash
# Remove the macOS quarantine flag from a downloaded AudioSync.app and open it.
#
# Downloading the release zip via a browser tags the app with
# com.apple.quarantine, which trips Gatekeeper ("Apple could not verify..."). On
# macOS 15+ there is no right-click -> Open bypass, so either use
# System Settings -> Privacy & Security -> Open Anyway, or run this script.
#
# Usage:
#   bash macos-first-run.sh [/path/to/AudioSync.app]
# Defaults to /Applications/AudioSync.app, then ./AudioSync.app.
set -euo pipefail

APP="${1:-}"
if [[ -z "$APP" ]]; then
  if [[ -d "/Applications/AudioSync.app" ]]; then
    APP="/Applications/AudioSync.app"
  elif [[ -d "./AudioSync.app" ]]; then
    APP="./AudioSync.app"
  else
    echo "AudioSync.app not found. Pass its path: bash macos-first-run.sh /path/to/AudioSync.app" >&2
    exit 1
  fi
fi

if [[ ! -d "$APP" ]]; then
  echo "Not found: $APP" >&2
  exit 1
fi

echo "Removing quarantine from: $APP"
xattr -dr com.apple.quarantine "$APP" || true
echo "Opening AudioSync..."
open "$APP"
echo "Done. If macOS still blocks it, approve once in System Settings > Privacy & Security."
