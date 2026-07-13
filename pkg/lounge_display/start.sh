#!/bin/bash
set -e

if [ -f /app/version.json ]; then
  # Try to parse string version or numeric version
  VERSION=$(grep -o '"version": "[^"]*"' /app/version.json | awk -F'"' '{print $4}')
  if [ -z "$VERSION" ]; then
    VERSION=$(grep -o '"version": [0-9]*' /app/version.json | awk '{print $2}')
  fi
  echo "lounge_display version: $VERSION"
else
  echo "lounge_display version: unknown"
fi

echo "Starting display_server..."
exec /app/display_server -dir /app/public -port 8080 -receiver /app/receiver
