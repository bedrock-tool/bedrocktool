#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/gomobile-bind.sh -o BedrockTool.framework
# Ensure you're on macOS. Install gomobile first:
#   go install golang.org/x/mobile/cmd/gomobile@latest
#   gomobile init

OUT="${1:-BedrockTool.framework}"

echo "Binding mobile package to iOS framework: $OUT"

gomobile bind -target=ios -o "$OUT" ./mobile

echo "Framework generated at $OUT"